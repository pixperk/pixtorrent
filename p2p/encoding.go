package p2p

import (
	"encoding/binary"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type BinaryDecoder struct{}

func (d *BinaryDecoder) Decode(r io.Reader, rpc *RPC) error {
	var len uint32
	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return err
	}

	//keep alive
	if len == 0 {
		rpc.Payload = nil
		return nil
	}

	buf := make([]byte, len)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}

	rpc.Payload = buf
	return nil
}
