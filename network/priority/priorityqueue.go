package priority

import (
	pb "dumbo_ms/structure"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

type SendBuf struct {
	MyID         int
	SbID         int
	N            int
	SendIP       string
	RcvIP        string
	Capacity     int
	Volume       int
	MemUse       int //mem useage now
	Con          net.Conn
	Buf          []pb.ConsOutMsg
	BufLock      sync.RWMutex
	CallHelpBuf  []pb.ConsOutMsg //calling help to others
	AssistBuf    []pb.ConsOutMsg //assistance msg to others
	MaxPriority  int
	ConsMsgOutCH chan pb.ConsOutMsg
}

func NewSendBuff(myid int, sbid int, num int, sendIP string, rcvip string, capacity int, volume int, consoutch chan pb.ConsOutMsg, con net.Conn) *SendBuf {
	fmt.Println("init sendbuf with mid", myid, "sid", sbid, "cap", capacity)
	return &SendBuf{
		MyID:         myid,
		SbID:         sbid,
		N:            num,
		SendIP:       sendIP,
		RcvIP:        rcvip,
		Capacity:     capacity,
		Volume:       volume,
		MemUse:       0,
		Con:          con,
		Buf:          make([]pb.ConsOutMsg, 0),
		MaxPriority:  0,
		ConsMsgOutCH: consoutch,
	}
}

func (sb *SendBuf) Start() {
	fmt.Println("start sendbuffer ", sb.SbID)
	go sb.AsyncSend()
	sb.HandleProtoMsg()
}

func (sb *SendBuf) HandleProtoMsg() {
	for {
		msg := <-sb.ConsMsgOutCH
		//fmt.Println("got a message to send in HandleProtoMsg")

		if msg.RevID != sb.SbID {
			//fmt.Println(msg)
			//fmt.Println("my sendbuffer ID", sb.SbID)
			continue
		}
		if msg.Type == 2 {
			sb.CallHelpBuf = append(sb.CallHelpBuf, msg)
		} else if msg.Type == 3 {
			sb.AssistBuf = append(sb.AssistBuf, msg)
		} else {
			sb.Push(msg)
		}
	}
}

func (sb *SendBuf) GetMaxPriority() int {
	return sb.MaxPriority
}

func (sb *SendBuf) Push(msg pb.ConsOutMsg) {
	sb.BufLock.Lock()
	defer sb.BufLock.Unlock()
	//fmt.Println("push a message")
	sb.MemUse += len(msg.Content) + 56
	sb.Buf = append(sb.Buf, msg)
	sort.Slice(sb.Buf, func(i, j int) bool { return sb.Buf[i].Priority < sb.Buf[j].Priority })
	if size := len(sb.Buf); size > sb.Capacity {
		sb.Buf = sb.Buf[size-sb.Capacity:]
	}

	for sb.MemUse > sb.Volume {
		firstElement := sb.Buf[0]
		firstElementSize := len(firstElement.Content) + 56
		sb.Buf = sb.Buf[1:]
		sb.MemUse -= firstElementSize
	}

}

func (sb *SendBuf) Pop() *pb.ConsOutMsg {
	sb.BufLock.Lock()
	defer sb.BufLock.Unlock()
	if size := len(sb.Buf); size < 1 {
		return nil
	} else {
		popmsg := sb.Buf[0]
		sb.Buf = sb.Buf[1:]
		//fmt.Println("pop a message")
		sb.MemUse -= len(popmsg.Content) + 56
		return &popmsg
	}
}

// async func that keep trying to send  msg
func (sb *SendBuf) AsyncSend() {
	//isreconnect := false
	hasLengthSent := false
	isMsgResend := false
	var msg *pb.ConsOutMsg
	var dialer *net.Dialer
	isLocalTest := sb.SendIP != sb.RcvIP

	if isLocalTest {
		tcpAddr, err := net.ResolveTCPAddr("tcp", sb.SendIP)
		dialer = &net.Dialer{LocalAddr: tcpAddr}
		if err != nil {
			panic("wrong localaddr")
		}
	}

	for {
		//pop msg from buffer
		if !isMsgResend {
			if callhalpBufLen := len(sb.CallHelpBuf); callhalpBufLen != 0 {
				msg = &sb.CallHelpBuf[callhalpBufLen-1]
				sb.CallHelpBuf = sb.CallHelpBuf[callhalpBufLen:]
			} else if assistBufLen := len(sb.AssistBuf); assistBufLen != 0 {
				msg = &sb.AssistBuf[assistBufLen-1]
				sb.AssistBuf = sb.AssistBuf[assistBufLen:]
			} else {
				msg = sb.Pop()
				//fmt.Println("pop a message to send")
				if msg == nil {
					time.Sleep(time.Millisecond * 50)
					continue
				}
			}

		} else {
			isMsgResend = false
		}
		//fmt.Println("pop a message to send")
		sendmsg := pb.ProtoMsg{SenderID: int32(msg.SendID), RcverID: int32(sb.SbID), Priority: int32(msg.Priority), Content: []byte(msg.Content), Type: int32(msg.Type)}
		sendbyte, err := proto.Marshal(&sendmsg)
		if err != nil {
			panic(err)
		}

		//try send
		//send length
		if !hasLengthSent {
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], uint32(len(sendbyte)))
			_, err = sb.Con.Write(buf[:])
			if err != nil {
				sb.Con.Close()
				//try reconnect
				isMsgResend = true
				for {
					if isLocalTest {
						con, err := dialer.Dial("tcp", sb.RcvIP)
						if err != nil {
							time.Sleep(time.Millisecond * 200)
						} else {
							sb.Con = con
							break
						}
					} else {
						con, err := net.Dial("tcp", sb.RcvIP+":12000")
						if err != nil {
							time.Sleep(time.Millisecond * 200)
						} else {
							sb.Con = con
							break
						}
					}
				}
				continue
			}
		}

		//send message
		_, err = sb.Con.Write(sendbyte)
		if err != nil {
			sb.Con.Close()
			//try reconnect
			isMsgResend = true
			hasLengthSent = true
			for {
				if isLocalTest {
					con, err := dialer.Dial("tcp", sb.RcvIP)
					if err != nil {
						time.Sleep(time.Millisecond * 200)
					} else {
						sb.Con = con
						break
					}
				} else {
					con, err := net.Dial("tcp", sb.RcvIP+":12000")
					if err != nil {
						time.Sleep(time.Millisecond * 200)
					} else {
						sb.Con = con
						break
					}
				}
			}
		} else {
			//fmt.Println("pop a message to send done")
			hasLengthSent = false
		}
	}
}

// ----------------------------------------------------------------------------------------------------------
// Receive buffer
type MsgCHwithPriority struct {
	Priority int
	MshCH    chan pb.ConsInMsg
}

type RcvBuf struct {
	MyID     int
	RbID     int
	N        int
	Capacity int //max quantity of messages
	Volume   int //max size of memory usage to store all messages
	MemUse   int //mem useage now

	Buf                     []pb.ConsInMsg //Buf[0] store msg with max priority
	BufLock                 sync.RWMutex
	MaxPriority             int
	NetCon                  chan net.Conn //switch con if old con failed
	ProtoMsgIn              chan pb.ConsInMsg
	RegisterCH              MsgCHwithPriority
	MaxPriorityOutCH        chan pb.MaxPrioritywithID
	WaitingAssistPriority   int
	IsWaitingForAssist      bool
	UpdateWaitingAssistLock sync.RWMutex
	AssistMsgCH             chan pb.AssistMsg
	CallHelpMsgCH           chan pb.ConsInMsg
}

// init
func NewRcvBuff(myid int, rbid int, num int, capacity int, volume int, netConns chan net.Conn, maxPriorityOutCH chan pb.MaxPrioritywithID, assistMsgCH chan pb.AssistMsg, callHelpMsgCH chan pb.ConsInMsg) *RcvBuf {
	fmt.Println("init rcvbuf with mid", myid, "rid", rbid, "cap", capacity, "vol", volume)
	return &RcvBuf{
		MyID:     myid,
		RbID:     rbid,
		N:        num,
		Capacity: capacity,
		Volume:   volume,
		MemUse:   0,
		//Buf:                   buf,
		MaxPriority:           -1,
		NetCon:                netConns,
		ProtoMsgIn:            make(chan pb.ConsInMsg, 10),
		RegisterCH:            MsgCHwithPriority{-1, nil},
		MaxPriorityOutCH:      maxPriorityOutCH,
		WaitingAssistPriority: -1,
		IsWaitingForAssist:    false,
		AssistMsgCH:           assistMsgCH,
		CallHelpMsgCH:         callHelpMsgCH,
	}
}

func (rb *RcvBuf) Start() {

	go rb.ListenProtoMsg()
	rb.HandleProtoMsg()

}

func (rb *RcvBuf) UpdateWaitingPriority(priority int) {
	rb.UpdateWaitingAssistLock.Lock()
	defer rb.UpdateWaitingAssistLock.Unlock()
	rb.WaitingAssistPriority = priority
	rb.IsWaitingForAssist = true
}

// return maxpriority
func (rb *RcvBuf) GetMaxPriority() int {
	return rb.MaxPriority
}

// read protomsg from con
func (rb *RcvBuf) ListenProtoMsg() {
	//first time
	msgcon := <-rb.NetCon
	switchcon := false
	hasLenRcv := false
	var msgLen int32
	for {
		//switch con if err
		if switchcon {
			msgcon = <-rb.NetCon
			switchcon = false
		}
		//read size
		if !hasLenRcv {
			//var size int32
			err1 := binary.Read(msgcon, binary.BigEndian, &msgLen)
			if err1 != nil {
				fmt.Println("error when read size from ", msgcon.RemoteAddr().String(), err1)
				switchcon = true

				continue
			}
		}
		// read data from the connection
		var buf = make([]byte, int(msgLen))
		tmpsize := msgLen
		for {
			var tmpbuf = make([]byte, int(tmpsize))
			//log.Println("start to read from conn")
			len, err := msgcon.Read(tmpbuf)
			//fmt.Println("reading------- size:", size, " size now:", n)
			if err != nil {
				fmt.Println("error when read msg from ", msgcon.RemoteAddr().String(), err)
				switchcon = true
				hasLenRcv = true
				break
			}
			if len <= 0 {
				fmt.Println("error when read msglen from ", msgcon.RemoteAddr().String())
				switchcon = true
				hasLenRcv = true
				break
			}

			buf = append(buf[:msgLen-tmpsize], tmpbuf[:len]...)
			if len < int(tmpsize) {
				tmpsize = tmpsize - int32(len)
			} else {
				hasLenRcv = false
				break
			}
		}

		if switchcon {
			continue
		} else {
			msg := buf[:]
			protomsg := pb.ProtoMsg{}
			err := proto.Unmarshal(msg, &protomsg)
			if err != nil {
				fmt.Println("error when unmarshal msg from ", msgcon.RemoteAddr().String(), err)
				continue
			}
			//fmt.Println("received a consensus message")
			if protomsg.RcverID == int32(rb.MyID) && protomsg.SenderID == int32(rb.RbID) {
				//fmt.Println("received a consensus message")
				rb.ProtoMsgIn <- pb.ConsInMsg{Priority: int(protomsg.Priority), SendID: int(protomsg.SenderID), RevID: int(protomsg.RcverID), Content: string(protomsg.Content), Type: int(protomsg.Type)}
			}
		}
	}
}

// handle protomsg from node ID. if protomsg.priority>maxpriority, creat a new channel and remove the oldest channel
func (rb *RcvBuf) HandleProtoMsg() {
	maxCallHelpPriority := -1
	for {
		protomsg := <-rb.ProtoMsgIn
		//handle assistance msg
		if protomsg.Type == 3 {
			rb.UpdateWaitingAssistLock.Lock()
			if rb.IsWaitingForAssist && protomsg.Priority == rb.WaitingAssistPriority {
				rb.AssistMsgCH <- pb.AssistMsg{SendID: protomsg.SendID, Priority: protomsg.Priority, Content: []byte(protomsg.Content)}
				rb.IsWaitingForAssist = false
			}
			rb.UpdateWaitingAssistLock.Unlock()
		} else if protomsg.Type == 2 {
			if protomsg.Priority > maxCallHelpPriority {
				maxCallHelpPriority = protomsg.Priority
				rb.CallHelpMsgCH <- protomsg
			}
		} else {
			rb.Push(protomsg)
		}
	}
}

func (rb *RcvBuf) Push(msg pb.ConsInMsg) {

	if msg.Priority > rb.MaxPriority {
		rb.MaxPriority = msg.Priority
		rb.MaxPriorityOutCH <- pb.MaxPrioritywithID{rb.RbID, msg.Priority}
	}

	if msg.Priority < rb.MaxPriority-3 {
		return
	}
	if rb.RegisterCH.Priority == msg.Priority {
		rb.RegisterCH.MshCH <- msg
	} else if msg.Priority > rb.RegisterCH.Priority {
		rb.BufLock.Lock()
		defer rb.BufLock.Unlock()
		rb.MemUse += len(msg.Content) + 56

		rb.Buf = append(rb.Buf, msg)
		sort.Slice(rb.Buf, func(i, j int) bool { return rb.Buf[i].Priority < rb.Buf[j].Priority })
		if size := len(rb.Buf); size > rb.Capacity {
			rb.Buf = rb.Buf[size-rb.Capacity:]
		}

		for rb.MemUse > rb.Volume {
			firstElement := rb.Buf[0]
			firstElementSize := len(firstElement.Content) + 56
			rb.Buf = rb.Buf[1:]
			rb.MemUse -= firstElementSize
		}
	}
}

func (rb *RcvBuf) RegisterMsgCHByPriority(priority int, consInMsgCH chan pb.ConsInMsg) {
	rb.BufLock.Lock()
	defer rb.BufLock.Unlock()

	//consInMsgCH := make(chan pb.ConsInMsg, rb.Capacity)
	rb.RegisterCH = MsgCHwithPriority{Priority: priority, MshCH: consInMsgCH}
	cutIndex := 0
	for i, msg := range rb.Buf {
		if msg.Priority == priority {
			rb.MemUse -= len(msg.Content) + 56
			cutIndex = i + 1
			consInMsgCH <- msg
		} else if msg.Priority > priority {
			break
		}
	}
	rb.Buf = rb.Buf[cutIndex:]
	//return consInMsgCH
}
