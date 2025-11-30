package p2p

const (
	MsgInterested    = 0x01
	MsgNotInterested = 0x02
	MsgRequestPiece  = 0x03
	MsgSendPiece     = 0x04
	MsgHave          = 0x05
	MsgBitfield      = 0x06
	MsgUnchoke       = 0x07
	MsgChoke         = 0x08
)

type From struct {
	PeerID [20]byte
	Addr   string
}

type RPC struct {
	From    From
	Payload []byte
}
