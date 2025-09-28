package torrentserver

import (
	"fmt"

	"github.com/pixperk/pixtorrent/p2p"
)

func (ts *TorrentServer) handleBitfieldAnnouncement(msg p2p.RPC, bitfield []byte) {
	fromaddr, fromid := msg.From.Addr, msg.From.PeerID
	fmt.Printf("[BITFIELD] from [Peer -> ID %x ; Addr %s], data: %x\n", fromid, fromaddr, bitfield)

	missing := ts.swarm.MissingPieces(bitfield)
	fmt.Printf("Missing pieces from this peer: %v\n", missing)

	//request first missing piece
	if len(missing) > 0 {
		if err := ts.RequestPiece(missing[0]); err != nil {
			fmt.Printf("failed to request piece %d from %x: %v\n", missing[0], fromid, err)
		}
	}
}
