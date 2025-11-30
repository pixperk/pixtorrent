package p2p

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const MaxMessageLength = 16 * 1024 * 1024

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type BinaryDecoder struct {
	br *bufio.Reader
}

func (d *BinaryDecoder) Decode(r io.Reader, rpc *RPC) error {
	// Cache buffered reader - safe because each connection has its own decoder
	if d.br == nil {
		if br, ok := r.(*bufio.Reader); ok {
			d.br = br
		} else {
			d.br = bufio.NewReaderSize(r, 64*1024)
		}
	}

	var length uint32
	if err := binary.Read(d.br, binary.BigEndian, &length); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		return err
	}

	if length == 0 {
		rpc.Payload = nil
		return nil
	}

	if length > MaxMessageLength {
		return fmt.Errorf("message length %d exceeds maximum allowed %d", length, MaxMessageLength)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(d.br, buf); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF // not enough data yet
		}
		return err
	}

	rpc.Payload = buf
	return nil
}
