package p2p

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
)

type HandshakeFunc func(Peer, [20]byte) error

func NOPHandshakeFunc(Peer, [20]byte) error { return nil }

// Handshake structure:
// <pstrlen><pstr><reserved><info_hash><peer_id>
// - pstrlen: 1 byte, length of pstr (19 for "piXTorrent protocol")
// - pstr: "piXTorrent protocol"
// - reserved: 8 bytes, all zero
// - info_hash: 20 bytes
// - peer_id: 20 bytes
func DefaultHandshakeFunc(peer Peer, infoHash [20]byte) error {
	// send handshake
	hs := buildHandshake(infoHash, peer.ID())
	_, err := peer.Write(hs)
	if err != nil {
		return err
	}

	// read peer handshake
	resp := make([]byte, len(hs))
	_, err = io.ReadFull(peer, resp)
	if err != nil {
		return err
	}

	// validate
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

	receivedPeerID := resp[1+pstrlen+8+20:]
	fmt.Println("handshake success with peer id:", string(receivedPeerID))
	return nil
}

func buildHandshake(infoHash [20]byte, peerID string) []byte {
	pstr := "piXTorrent protocol"
	buf := make([]byte, 49+len(pstr)) // 1 + pstrlen + 8 + 20 + 20

	buf[0] = byte(len(pstr))
	copy(buf[1:], []byte(pstr))
	// reserved 8 bytes already zero
	copy(buf[1+len(pstr)+8:], infoHash[:])
	copy(buf[1+len(pstr)+8+20:], []byte(peerID))

	return buf
}
