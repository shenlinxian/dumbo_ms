package honeybadger

import (
	"bufio"
	pb "dumbo_ms/structure"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
)

type HBConsensus struct {
	ID       int //my identity (start from 1)
	Num      int //number of nodes
	Fault    int
	Priority int                //priority of this consensus instance, which is round number here
	MsgInCH  chan pb.ConsInMsg  //sonsensus messages to others
	MsgOutCH chan pb.ConsOutMsg //consensus messages from others
	OutputCH chan pb.BlockInfo  //result of consensus
	Close    chan bool          //signal for closing this instance
}

func (hb *HBConsensus) Init(id int, num int, fault int, priority int, close chan bool) {
	hb.ID = id
	hb.Num = num
	hb.Fault = fault
	hb.Priority = priority
	hb.Close = close
}

func (hb *HBConsensus) Run(consMsgIn chan pb.ConsInMsg, consMsgOut chan pb.ConsOutMsg, input []byte, result chan pb.BlockInfo) {
	hb.MsgInCH = consMsgIn
	hb.MsgOutCH = consMsgOut
	hb.OutputCH = result
	//start python
	gopath := os.Getenv("GOPATH")

	inputStr := base64.StdEncoding.EncodeToString(input)
	cmd := exec.Command("python3", gopath+"/src/dumbo_ms/consensus/honeybadger/HoneyBadgerBFT/singlerun.py", strconv.Itoa(hb.Num), strconv.Itoa(hb.Fault), strconv.Itoa(hb.ID-1), strconv.Itoa(hb.Priority), inputStr)

	fmt.Println("execute command:", cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("Error creating stdin pipe:", err)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error1:", err)
		return
	}
	//go func() {
	if err := cmd.Start(); err != nil {
		fmt.Println("Error2:", err)
		return
	}
	//}()

	go hb.handleMsgInCH(stdin)
	go hb.handleMsgOutCH(stdout)

	//generate input to consensus
	//inputStr := base64.StdEncoding.EncodeToString(input)
	//consInput := pb.ConsInMsg{-1, 0, 0, inputStr, 1}
	//jsonData, err := json.Marshal(consInput)
	//if err != nil {
	//	panic(err)
	//}
	//_, err = stdin.Write(append(jsonData, '\n'))
	//if err != nil {
	//	fmt.Println("Error3:", err)
	//	return
	//}

	consEndCH := make(chan bool, 1)
	go func() {
		cmd.Wait()
		consEndCH <- true
	}()

	//wait for consensus ends itself or the main process decides to kill consensus thread
	select {
	case <-consEndCH:
		SafeClose(hb.Close)
	case <-hb.Close:
	}
	if err := cmd.Process.Kill(); err != nil {
		fmt.Println("Error killing process:", err)
	}

}

func (hb *HBConsensus) handleMsgInCH(stdin io.WriteCloser) {
	for {
		var consMsgIn pb.ConsInMsg
		select {
		case <-hb.Close:
			return
		default:
			select {
			case <-hb.Close:
				return
			case consMsgIn = <-hb.MsgInCH:
				//add a channel to receive tx, sendid=-1
			}
		}
		consMsgIn.SendID--
		consMsgIn.RevID--
		jsonData, err := json.Marshal(consMsgIn)
		if err != nil {
			panic(err)
		}
		stdin.Write(append(jsonData, '\n'))
		//stdin.Close()
	}
}

func (hb *HBConsensus) handleMsgOutCH(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)

	for {
		select {
		case <-hb.Close:
			return
		default:
		}
		if !scanner.Scan() {
			fmt.Println("error when scan output of python")
			fmt.Println(scanner.Err())
			continue
		}

		line := scanner.Bytes()
		var consMsgOut pb.ConsOutMsg
		if err := json.Unmarshal(line, &consMsgOut); err != nil {
			fmt.Println(line)
			panic(err)
			continue
		}

		if consMsgOut.RevID == -1 {
			content, err := base64.StdEncoding.DecodeString(consMsgOut.Content)
			if err != nil {
				fmt.Println("Error base decode content:", err)
				continue
			}
			//fmt.Println("receive output of consensus")
			hb.OutputCH <- pb.BlockInfo{hb.Priority, content}
		} else if consMsgOut.Priority >= 0 {
			//fmt.Println("receive consensus msg to others")
			//fmt.Println(consMsgOut)
			consMsgOut.RevID++
			consMsgOut.SendID++
			hb.MsgOutCH <- consMsgOut
		}
	}
}

func SafeClose(ch chan bool) {
	defer func() {
		if recover() != nil {
			// close(ch) panic occur
		}
	}()

	close(ch) // panic if ch is closed
}
