package p2p

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
)

type HandshakeFunc func(Peer, [20]byte, string, bool) error

func NOPHandshakeFunc(Peer, [20]byte, string, bool) error { return nil }

// Handshake structure:
// <pstrlen><pstr><reserved><info_hash><peer_id>
func DefaultHandshakeFunc(peer Peer, infoHash [20]byte, localID string, outbound bool) error {
	hs := buildHandshake(infoHash, localID)
	resp := make([]byte, len(hs))

	if outbound {
		if _, err := peer.Write(hs); err != nil {
			return err
		}
		if _, err := io.ReadFull(peer, resp); err != nil {
			return err
		}
	} else {
		if _, err := io.ReadFull(peer, resp); err != nil {
			return err
		}
		if _, err := peer.Write(hs); err != nil {
			return err
		}
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

	// extract remote peer ID
	receivedPeerID := string(resp[1+pstrlen+8+20:])
	if receivedPeerID == localID {
		return fmt.Errorf("connected to self, peer ID: %s", receivedPeerID)
	}
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
