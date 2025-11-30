package torrentserver

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixperk/pixtorrent/p2p"
)

func (ts *TorrentServer) handleBitfieldAnnouncement(msg p2p.RPC, bitfield []byte) {
	fromaddr, fromid := msg.From.Addr, msg.From.PeerID
	fmt.Printf("[BITFIELD] from [Peer -> ID %x ; Addr %s], data: %x\n", fromid, fromaddr, bitfield)

	ts.swarm.UpdatePeerBitfield(fromid, bitfield)

	rarestPieces := ts.swarm.GetRarestMissingPieces(bitfield)
	fmt.Printf("[RAREST PIECES] from [Peer -> ID %x ; Addr %s]: %v\n", fromid, fromaddr, rarestPieces)

	if len(rarestPieces) > 0 {
		peer, exists := ts.swarm.GetPeer(fromid)
		if !exists {
			return
		}

		// Send INTERESTED message to let peer know we want pieces
		if err := peer.Send([]byte{p2p.MsgInterested}); err != nil {
			fmt.Printf("failed to send interested to %x: %v\n", fromid, err)
			return
		}
		fmt.Printf("[SENT INTERESTED] to [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)

		// Store pending requests - will be sent when we receive UNCHOKE
		ts.storePendingRequests(fromid, rarestPieces)
	}
}

func (ts *TorrentServer) requestPiece(pieceIndex int) error {
	idxBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(idxBytes, uint32(pieceIndex))
	payload := append([]byte{p2p.MsgRequestPiece}, idxBytes...)
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to request piece from %s: %v\n", peer.ID(), err)
		}
	}
	return nil
}

func (ts *TorrentServer) requestPieceFromPeer(peer p2p.Peer, pieceIndex int) error {
	idxBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(idxBytes, uint32(pieceIndex))
	payload := append([]byte{p2p.MsgRequestPiece}, idxBytes...)
	return peer.Send(payload)
}

func (ts *TorrentServer) handlePieceRequest(msg p2p.RPC, pieceIdx int) {
	fromaddr, fromid := msg.From.Addr, msg.From.PeerID
	fmt.Printf("[REQUEST PIECE] from [Peer -> ID %x ; Addr %s], piece index: %d\n", fromid, fromaddr, pieceIdx)

	if ts.swarm.IsChoking(fromid) {
		fmt.Printf("[REJECTED] peer %x is choked, ignoring request for piece %d\n", fromid, pieceIdx)
		return
	}

	data, ok := ts.swarm.GetPiece(pieceIdx)
	if !ok {
		fmt.Printf("piece %d not found to send to %x\n", pieceIdx, fromid)
		return
	}

	idxBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(idxBytes, uint32(pieceIdx))
	payload := append([]byte{p2p.MsgSendPiece}, idxBytes...)
	payload = append(payload, data...)
	peer, exists := ts.swarm.GetPeer(fromid)
	if !exists {
		fmt.Printf("peer %x not found to send piece %d\n", fromid, pieceIdx)
		return
	}
	if err := peer.Send(payload); err != nil {
		fmt.Printf("failed to send piece %d to %x: %v\n", pieceIdx, fromid, err)
		return
	}
	ts.swarm.RecordUpload(fromid, int64(len(data)))
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
	if len(data) < 4 {
		fmt.Printf("[ERROR] piece data too short: %d bytes\n", len(data))
		return
	}
	index := int(binary.BigEndian.Uint32(data[:4]))
	pieceData := data[4:]

	if !ts.swarm.VerifyPiece(index, pieceData) {
		fmt.Printf("[REJECTED] piece %d failed hash verification from %x\n", index, msg.From.PeerID)
		return
	}

	ts.swarm.RecordDownload(msg.From.PeerID, int64(len(pieceData)))
	fmt.Printf("[RECEIVED PIECE] piece index %d with data %x from %x\n", index, pieceData, msg.From.PeerID)
	ts.swarm.AddPiece(index, pieceData)
	pieceIndex := index
	ts.announceHave(pieceIndex)

	go func() {
		if err := ts.UpdateTrackerStats(); err != nil {
			fmt.Printf("Failed to update tracker stats: %v\n", err)
		}
	}()

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

			go func() {
				if err := ts.AnnounceToTracker("completed"); err != nil {
					fmt.Printf("Failed to announce completion to tracker: %v\n", err)
				}
			}()
		}
	} else {
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
	idxBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(idxBytes, uint32(pieceIndex))
	payload := append([]byte{p2p.MsgHave}, idxBytes...)
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to announce have to %s: %v\n", peer.ID(), err)
		}
	}

	return nil
}

func (ts *TorrentServer) storePendingRequests(peerID [20]byte, pieces []int) {
	ts.pendingMu.Lock()
	defer ts.pendingMu.Unlock()
	ts.pendingRequests[peerID] = pieces
}

func (ts *TorrentServer) sendPendingRequests(peerID [20]byte) {
	ts.pendingMu.Lock()
	pieces, exists := ts.pendingRequests[peerID]
	if exists {
		delete(ts.pendingRequests, peerID)
	}
	ts.pendingMu.Unlock()

	if !exists || len(pieces) == 0 {
		return
	}

	peer, exists := ts.swarm.GetPeer(peerID)
	if !exists {
		return
	}

	for _, pieceIdx := range pieces {
		if err := ts.requestPieceFromPeer(peer, pieceIdx); err != nil {
			fmt.Printf("failed to request piece %d from %x: %v\n", pieceIdx, peerID, err)
		}
	}
}

func (ts *TorrentServer) AnnounceToTracker(event string) error {
	if ts.trackerClient == nil {
		return fmt.Errorf("tracker client not initialized")
	}

	left := int64(ts.swarm.MissingPiecesCount())

	resp, err := ts.trackerClient.Announce(ts.TrackerUrl, ts.TCPTransportOpts.InfoHash, left, event)
	if err != nil {
		return fmt.Errorf("failed to announce to tracker: %v", err)
	}

	fmt.Printf("[TRACKER ANNOUNCE] Event: %s, Interval: %ds, Peers returned: %d\n",
		event, resp.Interval, len(resp.Peers))

	return nil
}

func (ts *TorrentServer) ScrapeTracker() error {
	if ts.trackerClient == nil {
		return fmt.Errorf("tracker client not initialized")
	}

	resp, err := ts.trackerClient.Scrape(ts.TrackerUrl, ts.TCPTransportOpts.InfoHash)
	if err != nil {
		return fmt.Errorf("failed to scrape tracker: %v", err)
	}

	fmt.Printf("[TRACKER SCRAPE] Complete: %d, Incomplete: %d, Downloaded: %d\n",
		resp.Complete, resp.Incomplete, resp.Downloaded)

	return nil
}

func (ts *TorrentServer) UpdateTrackerStats() error {
	if ts.trackerClient == nil {
		return fmt.Errorf("tracker client not initialized")
	}

	uploaded := int64(len(ts.swarm.Peers()) * 1024)
	downloaded := int64(0)

	for i := 0; i < ts.swarm.NumPieces(); i++ {
		if _, ok := ts.swarm.GetPiece(i); ok {
			downloaded += 1024
		}
	}

	ts.trackerClient.UpdateStats(uploaded, downloaded)
	return ts.AnnounceToTracker("")
}
