package p2p

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

// Decoder defines the interface for decoding messages into RPC structs.
type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type BinaryDecoder struct{}

// Decode reads a length-prefixed message from the reader into rpc.
// If no data is available yet, it returns io.EOF instead of blocking forever.
func (d *BinaryDecoder) Decode(r io.Reader, rpc *RPC) error {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	// Non-blocking peek: check if at least 4 bytes are ready for the length
	if _, err := br.Peek(4); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF // no data yet
		}
		return err // some real error
	}

	var length uint32
	if err := binary.Read(br, binary.BigEndian, &length); err != nil {
		return err
	}

	// Keep-alive message (0 length)
	if length == 0 {
		rpc.Payload = nil
		return nil
	}

	// Check if the full payload is buffered
	if _, err := br.Peek(int(length)); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF // not enough data yet
		}
		return err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(br, buf); err != nil {
		return err
	}

	rpc.Payload = buf
	return nil
}
