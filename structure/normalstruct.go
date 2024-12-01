package structure

type RcvBufChan struct {
	Priority int
	Volume   int
	Channel  chan ConsInMsg
}

type ConsInMsg struct {
	SendID   int    `json:"sendid"`
	RevID    int    `json:"revid"`
	Priority int    `json:"priority"`
	Content  string `json:"content"`
	Type     int    `json:"type"` //1:consensus 2:callhelp 3:assistance
}

type ConsOutMsg struct {
	SendID   int    `json:"sendid"`
	RevID    int    `json:"revid"` //when RevID == -1, it's the result of consensus
	Priority int    `json:"priority"`
	Content  string `json:"content"`
	Type     int    `json:"type"` //1:consensus 2:callhelp 3:assistance
}

type MaxPrioritywithID struct {
	ID       int
	Priority int
}

type AssistMsg struct {
	SendID   int
	Priority int
	Content  []byte
}

type BlockInfo struct {
	Priority int
	Content  []byte
}

type HelpMsg struct {
	SendID   int
	RevID    int
	Priority int
	Content  []byte
}

type CallHelpMsg struct {
	SendID   int
	RevID    int
	Priority int
	Content  []byte
}

// fin type

type RBCOut struct {
	Value []byte
	ID    int
}

type BAMsg struct {
	ID        int
	RcvID     int
	MVBARound int
	BARound   int
	Loop      int
	Type      int
	Value     bool
	ConfValue []bool
}

type RBCMsg struct {
	ID     int
	RcvID  int
	Leader int
	Round  int
	Type   int //1 for ready 2 for echo
	Msglen int
	Root   []byte   //store root
	Values [][]byte //store path
}

type BCBlock struct {
	Payload []string `json:"payload"`
}
