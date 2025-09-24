package p2p

import "fmt"

type RPC struct {
	From    string
	Payload []byte
}

func (r RPC) String() string {
	if r.Payload == nil {
		return fmt.Sprintf("RPC{From: %s, Payload: nil}", r.From)
	}
	return fmt.Sprintf("RPC{From: %s, Payload: %x}", r.From, r.Payload)
}
