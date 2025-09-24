package p2p

import (
	"bytes"
	"testing"
)

func TestBinaryEncoderDecoder(t *testing.T) {
	encoder := &BinaryEncoder{}
	decoder := &BinaryDecoder{}

	// Test case 1: Normal message
	payload := []byte{0x04, 0x00, 0x00, 0x00, 0x01} // Example: have message (ID=4, piece index=1)
	rpc := &RPC{Payload: payload}

	encoded, err := encoder.Encode(rpc)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded: %x", encoded) // Hex string representation

	reader := bytes.NewReader(encoded)
	decodedRPC := &RPC{}
	err = decoder.Decode(reader, decodedRPC)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Original RPC: %s", rpc.String())
	t.Logf("Decoded RPC: %s", decodedRPC.String())

	if !bytes.Equal(decodedRPC.Payload, payload) {
		t.Errorf("Payload mismatch: got %x, want %x", decodedRPC.Payload, payload)
	}

	rpcKeepAlive := &RPC{Payload: nil}
	encodedKeepAlive, err := encoder.Encode(rpcKeepAlive)
	if err != nil {
		t.Fatalf("Encode keep-alive failed: %v", err)
	}

	readerKeepAlive := bytes.NewReader(encodedKeepAlive)
	decodedKeepAlive := &RPC{}
	err = decoder.Decode(readerKeepAlive, decodedKeepAlive)
	if err != nil {
		t.Fatalf("Decode keep-alive failed: %v", err)
	}

	if decodedKeepAlive.Payload != nil {
		t.Errorf("Keep-alive payload should be nil, got %x", decodedKeepAlive.Payload)
	}

	payloadEmpty := []byte{}
	rpcEmpty := &RPC{Payload: payloadEmpty}

	encodedEmpty, err := encoder.Encode(rpcEmpty)
	if err != nil {
		t.Fatalf("Encode empty payload failed: %v", err)
	}

	readerEmpty := bytes.NewReader(encodedEmpty)
	decodedEmpty := &RPC{}
	err = decoder.Decode(readerEmpty, decodedEmpty)
	if err != nil {
		t.Fatalf("Decode empty payload failed: %v", err)
	}

	if !bytes.Equal(decodedEmpty.Payload, payloadEmpty) {
		t.Errorf("Empty payload mismatch: got %x, want %x", decodedEmpty.Payload, payloadEmpty)
	}
}
