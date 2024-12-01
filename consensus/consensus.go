package consensus

import (
	pb "dumbo_ms/structure"
)

type Consensus interface {
	Init(id int, num int, priority int, close chan bool)
	Run(consMsgIn chan pb.ConsInMsg, consMsgOut chan pb.ConsOutMsg, input []byte, result chan []byte)
}
