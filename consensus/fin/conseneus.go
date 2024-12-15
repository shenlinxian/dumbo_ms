package fin

import (
	sfm "dumbo_ms/consensus/fin/MVBA"
	rbc "dumbo_ms/consensus/fin/RBC"
	pb "dumbo_ms/structure"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
)

type Fin struct {
	ID       int //my identity (start from 1)
	Num      int //number of nodes
	Fault    int
	Priority int                //priority of this consensus instance, which is round number here
	MsgInCH  chan pb.ConsInMsg  //sonsensus messages to others
	MsgOutCH chan pb.ConsOutMsg //consensus messages from others
	OutputCH chan pb.BlockInfo  //result of consensus
	Close    chan bool          //signal for closing this instance

	//message channel
	//in
	RBCMsgCH  []chan pb.RBCMsg
	RBCFMsgCH []chan pb.RBCMsg
	BAMsgCH   chan pb.BAMsg
	//out
	RBCMsgOutCH  chan pb.RBCMsg
	RBCFMsgOutCH chan pb.RBCMsg
	BAMsgOutCH   chan pb.BAMsg
}

func (fin *Fin) Init(id int, num int, fault int, priority int, close chan bool) {
	fin.ID = id
	fin.Num = num
	fin.Fault = fault
	fin.Priority = priority
	fin.Close = close

	fin.RBCMsgCH = make([]chan pb.RBCMsg, num)
	fin.RBCFMsgCH = make([]chan pb.RBCMsg, num)
	fin.BAMsgCH = make(chan pb.BAMsg, num*100)
	for i := 0; i < num; i++ {
		fin.RBCMsgCH[i] = make(chan pb.RBCMsg, num*9)
		fin.RBCFMsgCH[i] = make(chan pb.RBCMsg, num*9)
	}

	fin.RBCMsgOutCH = make(chan pb.RBCMsg, num*num*20)
	fin.RBCFMsgOutCH = make(chan pb.RBCMsg, num*num*20)
	fin.BAMsgOutCH = make(chan pb.BAMsg, num*100)
}

func (fin *Fin) Run(consMsgIn chan pb.ConsInMsg, consMsgOut chan pb.ConsOutMsg, input []byte, result chan pb.BlockInfo) {

	fin.MsgInCH = consMsgIn
	fin.MsgOutCH = consMsgOut

	go fin.MsgRouter()
	go fin.HandleMsgOut()

	done := make(chan bool)

	//fakedone := make(chan bool)

	ctoutputsCH := make(chan pb.RBCOut, fin.Num)
	go fin.start_ctrbcs(input, ctoutputsCH, done)

	ctrbcoutputs := make([]bool, fin.Num)
	ctrbcoutputscontent := make([][]byte, fin.Num)
	ctrbcdonecount := 0
	mvbaready := make(chan bool, 2)
	//wait for n-f ctrbc finished
	go func() {
		var output pb.RBCOut
		for {
			select {
			case <-done:
				return
			default:
				select {
				case <-done:
					return
				case output = <-ctoutputsCH:
					//fmt.Println("get an output from ready from", output.ID)
				}
				ctrbcdonecount++
				ctrbcoutputs[output.ID-1] = true
				ctrbcoutputscontent[output.ID-1] = output.Value
				if ctrbcdonecount == fin.Fault*2+1 {
					mvbaready <- true
				}
			}
		}
	}()

	<-mvbaready

	mvbainput := boolArrayToByteArray(ctrbcoutputs)
	time.Sleep(time.Millisecond * 100)
	//start sigfreemvba

	//fmt.Println("start a new signature free mvba of round", fin.Priority)

	mvba2order := make(chan []byte, 2)
	mvba := sfm.New_mvba(fin.Num, 1, fin.ID, fin.Priority, fin.RBCFMsgOutCH, fin.BAMsgOutCH, fin.RBCFMsgCH, fin.BAMsgCH, mvbainput, check_input_rbc, mvba2order, ctrbcoutputs)

	go mvba.Launch()

	output := <-mvba2order
	//fmt.Println("done a mvba of round ", fin.Priority)

	fin.wait_rbc_done(output, ctrbcoutputs)

	close(done)

	outputbools := byteArrayToBoolArray(output, fin.Num)

	//generate block
	var block pb.BCBlock
	for i, v := range outputbools {
		if v {
			block.Payload = append(block.Payload, string(ctrbcoutputscontent[i]))
		}
	}

	jsonData, err := json.Marshal(block)
	if err != nil {
		panic(err)
	}

	result <- pb.BlockInfo{Priority: fin.Priority, Content: jsonData}

	//fmt.Println("output:", outputbools)
}

func (fin *Fin) MsgRouter() {
	for {
		var msg pb.ConsInMsg
		select {
		case <-fin.Close:
			return
		default:
			select {
			case <-fin.Close:
				return
			case msg = <-fin.MsgInCH:
			}
		}

		finmsg := pb.FinMessage{}
		err := proto.Unmarshal([]byte(msg.Content), &finmsg)
		if err != nil {
			panic(err)
		}

		if finmsg.ID != int32(msg.SendID) || finmsg.RcvID != int32(msg.RevID) || finmsg.Round != int32(fin.Priority) {
			//missmatch message
			continue
		}

		switch finmsg.MsgType {
		case 1:
			//receive a RBC message
			rbcMsg := pb.RBCMsg{
				ID:     msg.SendID,
				RcvID:  msg.RevID,
				Leader: int(finmsg.Leader),
				Round:  fin.Priority,
				Type:   int(finmsg.RBCType),
				Msglen: int(finmsg.Msglen),
				Root:   finmsg.Root,
				Values: finmsg.Values,
			}

			//seperate messages by leader
			fin.RBCMsgCH[finmsg.Leader-1] <- rbcMsg
		case 2:
			//receive a RBC with finish message
			rbcMsg := pb.RBCMsg{
				ID:     msg.SendID,
				RcvID:  msg.RevID,
				Leader: int(finmsg.Leader),
				Round:  fin.Priority,
				Type:   int(finmsg.RBCType),
				Msglen: int(finmsg.Msglen),
				Root:   finmsg.Root,
				Values: finmsg.Values,
			}

			fin.RBCFMsgCH[finmsg.Leader-1] <- rbcMsg

		case 3:
			//receive a ba message
			baMsg := pb.BAMsg{
				ID:        msg.SendID,
				RcvID:     msg.RevID,
				MVBARound: fin.Priority,
				BARound:   int(finmsg.BARound),
				Loop:      int(finmsg.Loop),
				Type:      int(finmsg.BAType),
				Value:     finmsg.Value,
				ConfValue: finmsg.ConfValue,
			}
			//put it to BA channel

			fin.BAMsgCH <- baMsg
		default:
			//receive a wrong type message

			continue
		}

	}
}

func (fin *Fin) HandleMsgOut() {
	for {

		select {
		case <-fin.Close:
			return
		default:
			select {
			case <-fin.Close:
				return
			case msg := <-fin.RBCMsgOutCH:
				if msg.ID != fin.ID || msg.Round != fin.Priority {
					continue
				}
				finmsg := pb.FinMessage{
					MsgType: 1,
					ID:      int32(msg.ID),
					RcvID:   int32(msg.RcvID),
					Leader:  int32(msg.Leader),
					Round:   int32(msg.Round),
					RBCType: int32(msg.Type),
					Msglen:  int32(msg.Msglen),
					Root:    msg.Root,
					Values:  msg.Values,
				}
				msgByte, err := proto.Marshal(&finmsg)
				if err != nil {
					panic(err)
				}
				consOutMsg := pb.ConsOutMsg{
					SendID:   msg.ID,
					RevID:    msg.RcvID,
					Priority: msg.Round,
					Content:  string(msgByte),
					Type:     1,
				}
				fin.MsgOutCH <- consOutMsg

			case msg := <-fin.RBCFMsgOutCH:
				if msg.ID != fin.ID || msg.Round != fin.Priority {
					continue
				}
				finmsg := pb.FinMessage{
					MsgType: 2,
					ID:      int32(msg.ID),
					RcvID:   int32(msg.RcvID),
					Leader:  int32(msg.Leader),
					Round:   int32(msg.Round),
					RBCType: int32(msg.Type),
					Root:    msg.Root,
				}
				msgByte, err := proto.Marshal(&finmsg)
				if err != nil {
					panic(err)
				}
				consOutMsg := pb.ConsOutMsg{
					SendID:   msg.ID,
					RevID:    msg.RcvID,
					Priority: msg.Round,
					Content:  string(msgByte),
					Type:     1,
				}
				fin.MsgOutCH <- consOutMsg

			case msg := <-fin.BAMsgOutCH:
				if msg.ID != fin.ID || msg.MVBARound != fin.Priority {
					continue
				}
				finmsg := pb.FinMessage{
					MsgType:   3,
					ID:        int32(msg.ID),
					RcvID:     int32(msg.RcvID),
					Round:     int32(msg.MVBARound),
					BARound:   int32(msg.BARound),
					Loop:      int32(msg.Loop),
					BAType:    int32(msg.Type),
					Value:     msg.Value,
					ConfValue: msg.ConfValue,
				}
				msgByte, err := proto.Marshal(&finmsg)
				if err != nil {
					panic(err)
				}
				consOutMsg := pb.ConsOutMsg{
					SendID:   msg.ID,
					RevID:    msg.RcvID,
					Priority: msg.MVBARound,
					Content:  string(msgByte),
					Type:     1,
				}
				fin.MsgOutCH <- consOutMsg
			}
		}
	}
}

func byteArrayToBoolArray(arr []byte, size int) []bool {
	result := make([]bool, size)
	for i, b := range arr {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= len(result) {
				break
			}
			result[index] = (b >> j & 1) == 1
		}
	}
	return result
}

func boolArrayToByteArray(arr []bool) []byte {
	size := (len(arr) + 7) / 8
	result := make([]byte, size)
	for i, b := range arr {
		byteIndex := i / 8
		bitIndex := uint(i % 8)
		if b {
			result[byteIndex] |= 1 << bitIndex
		}
	}
	return result
}

func (fin *Fin) wait_rbc_done(input []byte, mine []bool) bool {
	result := make([]bool, len(mine))
	for i, b := range input {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= len(result) {
				break
			}
			result[index] = (b >> j & 1) == 1
		}
	}

	for {
		isvalid := true
		for i, b := range result {
			if b && !mine[i] {
				isvalid = false
				break
			}
		}
		if isvalid {
			return true
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
}

func (fin *Fin) start_ctrbcs(input []byte, output chan pb.RBCOut, done chan bool) {
	for i := 0; i < fin.Num; i++ {
		if i == fin.ID-1 {
			//start ctrbc leader
			newBcl := rbc.NewBroadcast_leader(i+1, i+1, 1, fin.Num, input, output, fin.RBCMsgCH[i], fin.RBCMsgOutCH, fin.Priority, done)
			go newBcl.Start()
		} else {
			//start ctrbc follower
			newBcf := rbc.NewBroadcast_follower(fin.ID, i+1, 1, fin.Num, output, fin.RBCMsgCH[i], fin.RBCMsgOutCH, fin.Priority, done)
			go newBcf.Start()
		}

	}
}

func check_input_rbc(input []byte, mine []bool, done chan bool) bool {
	result := make([]bool, len(mine))
	for i, b := range input {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= len(result) {
				break
			}
			result[index] = (b >> j & 1) == 1
		}
	}

	for {
		isvalid := true
		select {
		case <-done:
			return false
		default:
		}
		for i, b := range result {
			if b && !mine[i] {
				isvalid = false
				break
			}
		}
		if isvalid {
			return true
		} else {
			time.Sleep(time.Millisecond * 200)
		}
	}
}
