package p2p

import (
	"encoding/binary"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type Encoder interface {
	Encode(i *RPC) ([]byte, error)
}

type BinaryDecoder struct{}

type BinaryEncoder struct{}

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

func (e *BinaryEncoder) Encode(rpc *RPC) ([]byte, error) {
	buf := make([]byte, 4+len(rpc.Payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(rpc.Payload)))
	copy(buf[4:], rpc.Payload)
	return buf, nil
}
