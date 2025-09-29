package torrentserver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixperk/pixtorrent/p2p"
)

func (ts *TorrentServer) handleBitfieldAnnouncement(msg p2p.RPC, bitfield []byte) {
	fromaddr, fromid := msg.From.Addr, msg.From.PeerID
	fmt.Printf("[BITFIELD] from [Peer -> ID %x ; Addr %s], data: %x\n", fromid, fromaddr, bitfield)

	missing := ts.swarm.MissingPieces(bitfield)
	fmt.Printf("[MISSING PIECES] from [Peer -> ID %x ; Addr %s]: %x\n", fromid, fromaddr, missing)

	//request first missing piece
	if len(missing) > 0 {
		//send missing piece request at intervals of 500 ms
		for _, pieceIdx := range missing {
			err := ts.requestPiece(pieceIdx)
			if err != nil {
				fmt.Printf("failed to request piece %d from %x: %v\n", pieceIdx, fromid, err)
			}
		}
	}
}

func (ts *TorrentServer) requestPiece(pieceIndex int) error {
	payload := append([]byte{p2p.MsgRequestPiece}, byte(pieceIndex))
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to request piece from %s: %v\n", peer.ID(), err)
		}
	}
	return nil
}

func (ts *TorrentServer) handlePieceRequest(msg p2p.RPC, pieceIdx int) {
	fromaddr, fromid := msg.From.Addr, msg.From.PeerID
	fmt.Printf("[REQUEST PIECE] from [Peer -> ID %x ; Addr %s], piece index: %d\n", fromid, fromaddr, pieceIdx)
	data, ok := ts.swarm.GetPiece(pieceIdx)
	if !ok {
		fmt.Printf("piece %d not found to send to %x\n", pieceIdx, fromid)
		return
	}

	indexByte := byte(pieceIdx)
	payload := append([]byte{p2p.MsgSendPiece, indexByte}, data...)
	peer, exists := ts.swarm.GetPeer(fromid)
	if !exists {
		fmt.Printf("peer %x not found to send piece %d\n", fromid, pieceIdx)
		return
	}
	if err := peer.Send(payload); err != nil {
		fmt.Printf("failed to send piece %d to %x: %v\n", pieceIdx, fromid, err)
	}
	fmt.Printf("[SENT PIECE] piece index %d to [Peer -> ID %x ; Addr %s]\n", pieceIdx, fromid, fromaddr)
}

/* func (ts *TorrentServer) sendPiece(pieceIndex int, peerID [20]byte) error {
	data, ok := ts.swarm.GetPiece(pieceIndex)
	if !ok {
		return fmt.Errorf("piece %d not found", pieceIndex)
	}

	// Encode piece index as 4 bytes
	idxBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(idxBytes, uint32(pieceIndex))

	payload := append([]byte{p2p.MsgSendPiece}, idxBytes...)
	payload = append(payload, data...)

	peer, exists := ts.swarm.GetPeer(peerID)
	if !exists {
		return fmt.Errorf("peer %x not found", peerID)
	}

	if err := peer.Send(payload); err != nil {
		return fmt.Errorf("failed to send piece to %x: %v", peerID, err)
	}

	fmt.Printf("[SENT PIECE] piece index %d to [Peer -> ID %x]\n", pieceIndex, peerID)
	return nil
} */

func (ts *TorrentServer) ReconstructData() []byte {
	numPieces := ts.swarm.NumPieces()
	data := []byte{}
	for i := 0; i < numPieces; i++ {
		piece, ok := ts.swarm.GetPiece(i)
		if !ok {
			fmt.Printf("piece %d missing!\n", i)
			return nil
		}
		data = append(data, piece...)
	}
	return data
}

func (ts *TorrentServer) handlePiece(msg p2p.RPC, data []byte) {
	index := int(data[0])
	ts.swarm.AddPiece(index, data[1:])
	fmt.Printf("[RECEIVED PIECE] piece index %d with data %x from %x\n", index, data[1:], msg.From.PeerID)
	//set the piece as available in the swarm
	ts.swarm.AddPiece(index, data[1:])
	//announce to all peers that we have this piece now
	pieceIndex := index
	ts.announceHave(pieceIndex)

	if ts.swarm.AllPiecesReceived() {
		fmt.Println("All pieces received!")
		fullData := ts.ReconstructData()
		if fullData != nil {

			if err := os.MkdirAll(ts.RootDir, os.ModePerm); err != nil {
				fmt.Printf("Failed to create directory %s: %v\n", ts.RootDir, err)
				return
			}

			filePath := filepath.Join(ts.RootDir, fmt.Sprintf("%x.%s", ts.TCPTransportOpts.InfoHash, ts.FileFormat))

			if err := os.WriteFile(filePath, fullData, 0644); err != nil {
				fmt.Printf("Failed to write file %s: %v\n", filePath, err)
				return
			}

			fmt.Printf("Data successfully stored at %s\n", filePath)
		}
	} else {
		//send bitfield of available pieces to all peers
		bitfield := ts.swarm.Bitfield()
		payload := append([]byte{p2p.MsgBitfield}, bitfield...)
		for _, peer := range ts.swarm.Peers() {
			if err := peer.Send(payload); err != nil {
				fmt.Printf("failed to send bitfield to %s: %v\n", peer.ID(), err)
			}
		}
	}
}

func (ts *TorrentServer) announceHave(pieceIndex int) error {
	payload := append([]byte{p2p.MsgHave}, byte(pieceIndex))
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to announce have to %s: %v\n", peer.ID(), err)
		}
	}

	return nil
}
