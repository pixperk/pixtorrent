package p2p

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
)

type HandshakeFunc func(Peer, [20]byte, string) error

func NOPHandshakeFunc(Peer, [20]byte, string) error { return nil }

// Handshake structure:
// <pstrlen><pstr><reserved><info_hash><peer_id>
// - pstrlen: 1 byte, length of pstr (19 for "piXTorrent protocol")
// - pstr: "piXTorrent protocol"
// - reserved: 8 bytes, all zero
// - info_hash: 20 bytes
// - peer_id: 20 bytes
func DefaultHandshakeFunc(peer Peer, infoHash [20]byte, localID string) error {
	// send handshake with our own peer ID
	hs := buildHandshake(infoHash, localID)
	_, err := peer.Write(hs)
	if err != nil {
		return err
	}

	// read remote peer handshake
	resp := make([]byte, len(hs))
	_, err = io.ReadFull(peer, resp)
	if err != nil {
		return err
	}

	// validate protocol
	pstrlen := int(resp[0])
	pstr := string(resp[1 : 1+pstrlen])
	if pstr != "piXTorrent protocol" {
		return fmt.Errorf("unexpected protocol: %s", pstr)
	}

	receivedInfoHash := resp[1+pstrlen+8 : 1+pstrlen+8+20]
	if !bytes.Equal(receivedInfoHash, infoHash[:]) {
		return fmt.Errorf("infohash mismatch: expected %s got %s",
			hex.EncodeToString(infoHash[:]),
			hex.EncodeToString(receivedInfoHash))
	}

	// extract remote peer ID and assign to peer
	receivedPeerID := string(resp[1+pstrlen+8+20:])
	peer.SetID(receivedPeerID)
	fmt.Println("handshake success with peer id:", receivedPeerID)
	return nil
}

func buildHandshake(infoHash [20]byte, localPeerID string) []byte {
	pstr := "piXTorrent protocol"
	buf := make([]byte, 49+len(pstr)) // 1 + pstrlen + 8 + 20 + 20

	buf[0] = byte(len(pstr))
	copy(buf[1:], []byte(pstr))
	// reserved 8 bytes already zero
	copy(buf[1+len(pstr)+8:], infoHash[:])
	copy(buf[1+len(pstr)+8+20:], []byte(localPeerID))

	return buf
}
