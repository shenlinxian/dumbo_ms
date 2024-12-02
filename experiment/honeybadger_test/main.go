package main

import (
	"bufio"
	hb "dumbo_ms/consensus/honeybadger"
	db "dumbo_ms/database"
	log "dumbo_ms/log"
	net "dumbo_ms/network"
	nn "dumbo_ms/network/normal"
	nl "dumbo_ms/network/normallimitmemory"
	pn "dumbo_ms/network/priority"
	pb "dumbo_ms/structure"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type HbTest struct {
	ID                int
	N                 int    `yaml:"N"`
	Fault             int    `yaml:"Fault"`
	IsLocalTest       bool   `yaml:"IsLocalTest"`
	NetWorkType       int    `yaml:"NetWorkType"`
	Byz               bool   `yaml:"Byz"`
	ByzRate           int    `yaml:"ByzRate"`
	ByzTarget         int    `yaml:"ByzTarget"`
	EnableMemProtect  bool   `yaml:"EnableMemProtect"`
	Priority          int    `yaml:"Priority"`
	BatchSize         int    `yaml:"BatchSize"`
	TXSize            int    `yaml:"TXSize"`
	IpPath            string `yaml:"IpPath"`
	LogFile           string `yaml:"LogFile"`
	DBPath            string `yaml:"DBPath"`
	MaxSendBufferSize int    `yaml:"MaxSendBufferSize"`
	MaxRcvBufferSize  int    `yaml:"MaxRcvBufferSize"`

	HbLog log.MsLog
	HbDB  db.MsDB
	HbNet net.MsNet

	DBBlockCH chan pb.BlockInfo //store block in database

	InputCH                chan []byte
	MsgOutCH               chan pb.ConsOutMsg
	ConsMsgOutCH           chan pb.ConsOutMsg
	SafePriorityCH         chan int
	MyCallHelpCH           chan int
	AssitBlockFromOthersCH chan pb.BlockInfo
	OthersCallHelpMsgCH    chan pb.ConsInMsg
}

func main() {
	var idf = flag.Int("id", 0, "-id")
	flag.Parse()
	id := *idf

	fmt.Println("running test for honeybadger with id=", id)
	fmt.Println("start loading configure file")
	gopath := os.Getenv("GOPATH")
	readBytes, err := os.ReadFile(gopath + "/src/dumbo_ms/config/configure.yaml")
	if err != nil {
		panic(err)
	}

	var hbTest HbTest
	err = yaml.Unmarshal(readBytes, &hbTest)
	if err != nil {
		panic(err)
	}
	hbTest.ID = id
	fmt.Println("end loading configure file")

	//init channel

	fmt.Println("start init log")
	//init log
	logFileName := fmt.Sprintf("/ms%d.log", id)
	hbTest.HbLog.Init(gopath + hbTest.LogFile + logFileName)

	fmt.Println("end init log")

	fmt.Println("start init database")
	//init database
	hbTest.DBBlockCH = make(chan pb.BlockInfo, 100)
	go hbTest.HbDB.Init(gopath+hbTest.DBPath+fmt.Sprintf("/%d", hbTest.ID), hbTest.HbLog, hbTest.DBBlockCH)
	fmt.Println("end init database")

	fmt.Println("start init network")
	//init network
	switch hbTest.NetWorkType {
	case 1:
		hbTest.HbNet = &nn.NormalNetwork{}
	case 2:
		hbTest.HbNet = &nl.NormalNetworkLimit{}
	case 3:
		hbTest.HbNet = &pn.PriorityNetwork{}
	}
	//read ips, ipSrc: source port, ipDrc: direction port
	ipPath := fmt.Sprintf("%s%s", gopath, hbTest.IpPath)

	hbTest.MsgOutCH = make(chan pb.ConsOutMsg, 200)
	hbTest.ConsMsgOutCH = make(chan pb.ConsOutMsg, 200)
	hbTest.SafePriorityCH = make(chan int, 100)
	hbTest.MyCallHelpCH = make(chan int, 100)
	hbTest.AssitBlockFromOthersCH = make(chan pb.BlockInfo, 100)
	hbTest.OthersCallHelpMsgCH = make(chan pb.ConsInMsg, hbTest.N*3)
	hbTest.HbNet.Init(hbTest.ID, hbTest.N, hbTest.Fault, ipPath, hbTest.IsLocalTest, hbTest.MaxSendBufferSize, 0, hbTest.MaxRcvBufferSize, 0, hbTest.MsgOutCH, hbTest.SafePriorityCH, hbTest.MyCallHelpCH, hbTest.AssitBlockFromOthersCH, hbTest.OthersCallHelpMsgCH)
	fmt.Println("end init network")

	fmt.Println("Start running honeybadger consensus: n=", hbTest.N)
	//generate input
	hbTest.InputCH = make(chan []byte, 50)
	go hbTest.GenerateInput()
	go hbTest.HandleConsOutMsg()
	//loop for consensus runing
	for {
		roundPriority := hbTest.Priority
		var hb hb.HBConsensus
		roundClose := make(chan bool)
		hb.Init(id, hbTest.N, hbTest.Fault, roundPriority, roundClose)

		//regesiter msgin channel
		msgInCH := make(chan pb.ConsInMsg, hbTest.MaxRcvBufferSize*2)
		input := <-hbTest.InputCH
		resultCH := make(chan pb.BlockInfo, 2)
		hbTest.HbNet.Receive(roundPriority, msgInCH)
		hb.Run(msgInCH, hbTest.ConsMsgOutCH, input, resultCH)

		//wait for jump signal or consensus ends
		breakSignal := false
		for {
			select {
			case safePriority := <-hbTest.SafePriorityCH:
				if safePriority > roundPriority+1 {
					hbTest.Priority = hbTest.CallHelp(safePriority) + 1
					breakSignal = true
				}
			case result := <-resultCH:
				hbTest.DBBlockCH <- pb.BlockInfo{Priority: result.Priority, Content: result.Content}
				hbTest.Priority++
				breakSignal = true
			}
			if breakSignal {
				break
			}
		}
		fmt.Println("Finished round:", hbTest.Priority)
		hbTest.HbLog.Info(fmt.Sprintln("Finished round:", hbTest.Priority))
	}
}

func (hbtest *HbTest) HandleConsOutMsg() {
	if hbtest.Byz && hbtest.ID <= hbtest.Fault {
		hbtest.HandleConsOutMsgWithAttack()
	} else {
		hbtest.HandleConsOutMsgNormal()
	}
}

func (hbtest *HbTest) HandleConsOutMsgNormal() {
	for {
		msg := <-hbtest.ConsMsgOutCH
		hbtest.MsgOutCH <- msg
	}
}

func (hbtest *HbTest) HandleConsOutMsgWithAttack() {
	for {
		msg := <-hbtest.ConsMsgOutCH
		hbtest.MsgOutCH <- msg
		if msg.RevID != hbtest.ByzTarget {
			continue
		}
		msg.Priority += 10000
		for i := 0; i < hbtest.ByzRate; i++ {
			msg.Priority++
			hbtest.MsgOutCH <- msg
		}
	}
}

// call for all miss blocks below than safePriority
func (hbtest *HbTest) CallHelp(safePriority int) int {
	start := hbtest.Priority
	end := safePriority
	quitSig := false
	for {
		select {
		case newend := <-hbtest.SafePriorityCH:
			start = end
			end = newend
		default: //catch on newest block
			quitSig = true
		}
		if quitSig {
			return end - 2
		}
		//send callhelp
		for i := start; i < end; i++ {
			hbtest.MyCallHelpCH <- i
		}
		//wait for help block
		for {
			block := <-hbtest.AssitBlockFromOthersCH
			hbtest.DBBlockCH <- block
			if block.Priority == end-2 {
				break
			}
		}
	}
}

func (hbtest *HbTest) HandleOthersCallhelp() {

	for {
		callHelpMsg := <-hbtest.OthersCallHelpMsgCH
		if callHelpMsg.Priority > hbtest.Priority {
			continue
		}
		//find this block
		block := hbtest.HbDB.FindWithPriority(callHelpMsg.Priority)
		if block == nil {
			continue
		}
		//generate assist message
		assist := pb.ConsOutMsg{SendID: hbtest.ID, RevID: callHelpMsg.SendID, Priority: callHelpMsg.Priority, Content: string(block), Type: 3}
		hbtest.MsgOutCH <- assist
	}
}

func ReadIPs(path string, num int) []string {
	fi, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return nil
	}

	br := bufio.NewReader(fi)
	ips := make([]string, num)
	for i := 0; i < num; i++ {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		ips[i] = string(a)
	}
	fi.Close()
	return ips

}

func (hb *HbTest) GenerateInput() {
	i := 0
	for {
		id1 := strconv.Itoa(i)
		id2 := strconv.Itoa(hb.ID)
		txs := []byte(id1 + id2)
		var tx []byte
		if len := len(txs); len < hb.TXSize {
			padding := make([]byte, hb.TXSize-len)
			tx = append(txs, padding...)
		} else {
			tx = txs[len-hb.TXSize:]
		}
		hb.InputCH <- tx
	}

}
