package p2p

const (
	MsgInterested   = 0x01
	MsgRequestPiece = 0x02
	MsgSendPiece    = 0x03
	MsgHave         = 0x04
	MsgBitfield     = 0x05
	MsgUnchoke      = 0x06
	MsgChoke        = 0x07
)

type RPC struct {
	From    string
	Payload []byte
}
