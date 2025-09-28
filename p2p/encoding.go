package p2p

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type BinaryDecoder struct{}

func (d *BinaryDecoder) Decode(r io.Reader, rpc *RPC) error {
	br, ok := r.(*bufio.Reader)
	if !ok {

		br = bufio.NewReaderSize(r, 64*1024) // 64KB buffer
	}

	var length uint32
	if err := binary.Read(br, binary.BigEndian, &length); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		return err
	}

	if length == 0 {
		rpc.Payload = nil
		return nil
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(br, buf); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF // not enough data yet
		}
		return err
	}

	rpc.Payload = buf
	return nil
}
