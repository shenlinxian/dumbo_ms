package network

import (
	pb "dumbo_ms/structure"
)

type MsNet interface {
	Init(id int, num int, fault int, ippath string, isLocalTest bool, maxsendbufsize int, maxsendbufferquantity int, maxrcvbufsize int, maxrcvbufferquantity int, sendmsgch chan pb.ConsOutMsg, safePriorityUpdateCH chan int, myCallHelpCH chan int, assistBlockOutCH chan pb.BlockInfo, callHelpMsgFromOthersCH chan pb.ConsInMsg)

	Receive(priority int, consInMsgCH chan pb.ConsInMsg)
}
