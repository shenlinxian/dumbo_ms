package priority

import (
	"bufio"
	pb "dumbo_ms/structure"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type PriorityNetwork struct {
	ID            int
	N             int
	Fault         int
	Height        int
	IPpath        string
	myIPs         []string //local ips using to dial others
	otherIPs      []string //other ips that will dial me
	IPd           []string //ips dial for
	Cons          []chan net.Conn
	IsLocalTest   bool
	SendBuffer    []SendBuf
	RcvBuffer     []RcvBuf
	SendMsgCH     chan pb.ConsOutMsg
	SubSendMsgCHs []chan pb.ConsOutMsg

	MaxSendBufferSize         int
	MaxSendBufferQuantity     int
	MaxRcvBufferSize          int
	MaxRcvBufferQuantity      int
	MaxPriorities             []int
	SafePriority              int
	PrioritiesOutCH           chan pb.MaxPrioritywithID
	SafePriorityUpdateCH      chan int
	MyCallHelpCH              chan int
	AssistMsgFromOthersCH     chan pb.AssistMsg //assistance msg from others
	AssisBlockFromOthersOutCH chan pb.BlockInfo //f+1 same block in a priority can be output
	CallHelpMsgFromOthersCH   chan pb.ConsInMsg //callhelp msg from others
	MaxCallHelpPriorities     []int             //max priorities for every node they call for help

	InitWg sync.WaitGroup
}

func (pn *PriorityNetwork) Init(id int, num int, fault int, ippath string, isLocal bool, maxsendbufsize int, maxsendbufferquantity int, maxrcvbufsize int, maxrcvbufferquantity int, sendmsgch chan pb.ConsOutMsg, safePriorityUpdateCH chan int, myCallHelpCH chan int, assistBlockOutCH chan pb.BlockInfo, callHelpMsgFromOthersCH chan pb.ConsInMsg) {
	pn.ID = id
	pn.N = num
	pn.Fault = fault
	pn.Height = 0
	pn.IPpath = ippath
	pn.IsLocalTest = isLocal
	//pn.IPs = ips
	//pn.IPd = ipd
	//pn.ServerIP = serviceip
	pn.Cons = make([]chan net.Conn, num)
	for i := 0; i < num; i++ {
		pn.Cons[i] = make(chan net.Conn, 10)
	}
	pn.SendBuffer = make([]SendBuf, num)
	pn.RcvBuffer = make([]RcvBuf, num)
	pn.MaxSendBufferSize = maxsendbufsize
	pn.MaxSendBufferQuantity = maxsendbufferquantity
	pn.MaxRcvBufferSize = maxrcvbufsize
	pn.MaxRcvBufferQuantity = maxrcvbufferquantity
	pn.MaxPriorities = make([]int, num)
	pn.SafePriority = -1
	pn.PrioritiesOutCH = make(chan pb.MaxPrioritywithID, num*2)
	pn.SafePriorityUpdateCH = safePriorityUpdateCH
	pn.MyCallHelpCH = myCallHelpCH
	pn.SendMsgCH = sendmsgch
	pn.SubSendMsgCHs = make([]chan pb.ConsOutMsg, num)
	for i := 0; i < num; i++ {
		pn.SubSendMsgCHs[i] = make(chan pb.ConsOutMsg, 100)
	}
	pn.AssistMsgFromOthersCH = make(chan pb.AssistMsg, pn.N*3)
	pn.AssisBlockFromOthersOutCH = assistBlockOutCH
	pn.CallHelpMsgFromOthersCH = callHelpMsgFromOthersCH

	//listen for bft nodes

	pn.ReadIPs()

	pn.InitWg.Add(2)
	go pn.InitSendBuf()
	go pn.InitRcvBuf()
	go pn.HandleConn()
	pn.InitWg.Wait()
	go pn.HandlePriorities()
	go pn.SendMsgRouter()
	go pn.HandleOwnCallHelp()
	time.Sleep(time.Second * 5)

}

func (pn *PriorityNetwork) ReadIPs() {
	ipPath := fmt.Sprintf("%sdrtips.txt", pn.IPpath)
	ipDrt := ReadIPs(ipPath, pn.N)
	if ipDrt == nil {
		panic("wrong read ips")
	}

	if pn.IsLocalTest {
		ipPath = fmt.Sprintf("%sip%d/myips.txt", pn.IPpath, pn.ID)
		myIpSrc := ReadIPs(ipPath, pn.N)
		if myIpSrc == nil {
			panic("wrong read ips")
		}

		ipPath = fmt.Sprintf("%sip%d/otherips.txt", pn.IPpath, pn.ID)
		otherIpSrc := ReadIPs(ipPath, pn.N)
		if otherIpSrc == nil {
			panic("wrong read ips")
		}

		pn.myIPs = myIpSrc
		pn.otherIPs = otherIpSrc
	}

	pn.IPd = ipDrt
}

func (pn *PriorityNetwork) SendMsgRouter() {
	for {
		sendMsg := <-pn.SendMsgCH

		//fmt.Println("got a message to send in SendMsgRouter")
		if sendMsg.RevID < 1 || sendMsg.RevID > pn.N {
			continue
		}

		pn.SubSendMsgCHs[sendMsg.RevID-1] <- sendMsg
	}
}

// update safe priority
func (pn *PriorityNetwork) HandlePriorities() {

	for {
		//var priorityStruct pb.MaxPrioritywithID

		priorityStruct := <-pn.PrioritiesOutCH
		pn.MaxPriorities[priorityStruct.ID-1] = priorityStruct.Priority

		if priorityStruct.Priority > pn.SafePriority {
			tmpSafePriority := findFthLargest(pn.MaxPriorities, pn.Fault+1)
			if tmpSafePriority > pn.SafePriority {
				pn.SafePriority = tmpSafePriority
				pn.SafePriorityUpdateCH <- pn.SafePriority
			}
		}
	}

}

// listen and route con
func (pn *PriorityNetwork) HandleConn() {
	fmt.Println("start listen ip:", pn.IPd[pn.ID-1])
	var tcpAddr *net.TCPAddr
	tcpAddr, _ = net.ResolveTCPAddr("tcp", pn.IPd[pn.ID-1]+":12000")
	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err)
	}

	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			panic(err)
		}
		fmt.Println("A replica connected :" + tcpConn.RemoteAddr().String())

		isIPMatch := false

		for i := 0; i < pn.N; i++ {
			if pn.IsLocalTest {
				if tcpConn.RemoteAddr().String() == pn.otherIPs[i] {
					pn.Cons[i] <- tcpConn
					isIPMatch = true
					break
				}
			} else {
				tcpAddr := tcpConn.RemoteAddr().String()
				ip := strings.Split(tcpAddr, ":")[0]
				if ip == pn.IPd[i] {
					pn.Cons[i] <- tcpConn
					isIPMatch = true
					break
				}
			}

		}
		if !isIPMatch {
			fmt.Println("Unknown node connected", tcpConn.RemoteAddr())
			tcpConn.Close()
		}

	}

}

func (pn *PriorityNetwork) InitSendBuf() {
	//fmt.Println("start init sendbuffer")
	var sbwg sync.WaitGroup

	for i := 1; i <= pn.N; i++ {
		sbwg.Add(1)
		go func(id int) {
			defer sbwg.Done()
			for {
				if pn.IsLocalTest {
					tcpAddr, err := net.ResolveTCPAddr("tcp", pn.myIPs[id-1])
					dialer := &net.Dialer{LocalAddr: tcpAddr}
					if err != nil {
						panic("wrong localaddr")
					}
					conn, err := dialer.Dial("tcp", pn.IPd[id-1])
					fmt.Println("Dialing ", pn.IPd[id-1])
					if err != nil {
						//conn.Close()
						fmt.Println("err when init senfbuf", err)
						time.Sleep(time.Millisecond * 900)
					} else {
						fmt.Println("Dial ", pn.IPd[id-1], "done")
						//init responding send buf
						pn.SendBuffer[id-1] = *NewSendBuff(pn.ID, id, pn.N, pn.myIPs[id-1], pn.IPd[id-1], pn.MaxSendBufferSize/pn.N, pn.MaxSendBufferQuantity, pn.SubSendMsgCHs[id-1], conn)
						go pn.SendBuffer[id-1].Start()
						break
					}
				} else {
					ipAddr := pn.IPd[id-1] + ":12000"
					conn, err := net.Dial("tcp", ipAddr)
					fmt.Println("Dialing ", pn.IPd[id-1])
					if err != nil {
						//conn.Close()
						fmt.Println("err when init senfbuf", err)
						time.Sleep(time.Millisecond * 900)
					} else {
						fmt.Println("Dial ", pn.IPd[id-1], "done")
						//init responding send buf
						pn.SendBuffer[id-1] = *NewSendBuff(pn.ID, id, pn.N, pn.IPd[id-1], pn.IPd[id-1], pn.MaxSendBufferSize/pn.N, pn.MaxSendBufferQuantity, pn.SubSendMsgCHs[id-1], conn)
						go pn.SendBuffer[id-1].Start()
						break
					}

				}

			}
		}(i)
	}
	sbwg.Wait()
	pn.InitWg.Done()
	//fmt.Println("done init sendbuffer")
}

func (pn *PriorityNetwork) InitRcvBuf() {
	//fmt.Println("start init rcvbuffer")
	var rbwg sync.WaitGroup
	for i := 1; i <= pn.N; i++ {
		rbwg.Add(1)
		go func(id int) {
			defer rbwg.Done()
			pn.RcvBuffer[id-1] = *NewRcvBuff(pn.ID, id, pn.N, pn.MaxRcvBufferSize/pn.N, pn.MaxRcvBufferQuantity, pn.Cons[id-1], pn.PrioritiesOutCH, pn.AssistMsgFromOthersCH, pn.CallHelpMsgFromOthersCH)
			go pn.RcvBuffer[id-1].Start()
		}(i)
	}
	rbwg.Wait()
	pn.InitWg.Done()
	//fmt.Println("done init rcvbuffer")
}

// given a priority and a msg channel, pn will put all received msgs with this priority in thie channel
func (pn *PriorityNetwork) Receive(priority int, consInMsgCH chan pb.ConsInMsg) {
	for i := 0; i < pn.N; i++ {
		go pn.RcvBuffer[i].RegisterMsgCHByPriority(priority, consInMsgCH)
	}
}

func findFthLargest(arr []int, f int) int {
	if f > 0 && f <= len(arr) {
		// 复制原数组
		arrCopy := make([]int, len(arr))
		copy(arrCopy, arr)

		// 对复制的数组进行排序
		sort.Sort(sort.Reverse(sort.IntSlice(arrCopy)))

		// 返回第f大的元素
		return arrCopy[f-1]
	}
	fmt.Println("Invalid input")
	return -1
}

func (pn *PriorityNetwork) HandleOwnCallHelp() {

	//generate my callhelp msg
	for {
		callhelpPriority := <-pn.MyCallHelpCH
		myCallHelpMsg := pb.ConsOutMsg{SendID: pn.ID, Priority: callhelpPriority, Type: 2}
		for i := 0; i < pn.N; i++ {
			if i+1 != pn.ID {
				myCallHelpMsg.RevID = i + 1
				pn.RcvBuffer[i].UpdateWaitingPriority(callhelpPriority)
				pn.SubSendMsgCHs[i] <- myCallHelpMsg
			}
		}

		recordIDs := make(map[int]bool)
		recordBlocks := make(map[string]int)
		for {
			assistBlockMsg := <-pn.AssistMsgFromOthersCH
			if assistBlockMsg.Priority == callhelpPriority {
				_, ok := recordIDs[assistBlockMsg.SendID]
				if ok {
					continue
				}
				recordIDs[assistBlockMsg.SendID] = true
				count, ok := recordBlocks[string(assistBlockMsg.Content)]
				if ok {
					recordBlocks[string(assistBlockMsg.Content)] = count + 1
				} else {
					recordBlocks[string(assistBlockMsg.Content)] = 1
				}
				if recordBlocks[string(assistBlockMsg.Content)] == pn.Fault+1 {
					block := pb.BlockInfo{Priority: assistBlockMsg.Priority, Content: assistBlockMsg.Content}
					pn.AssisBlockFromOthersOutCH <- block
					break
				}

			}
		}

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
