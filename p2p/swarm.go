package p2p

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"sync"
)

const (
	MaxUnchokedPeers     = 4
	OptimisticUnchokeInterval = 3
)

type Swarm struct {
	peers        map[[20]byte]Peer
	peerStates   map[[20]byte]*PeerState
	peerBitfields map[[20]byte][]byte
	mu           sync.Mutex
	infoHash     [20]byte
	pieces       *PieceManager
	localPeerID  [20]byte

	optimisticPeer  [20]byte
	unchokeRound    int
}

func NewSwarm(localPeerId [20]byte, infoHash [20]byte, pieceMgr *PieceManager) *Swarm {
	return &Swarm{
		peers:         make(map[[20]byte]Peer),
		peerStates:    make(map[[20]byte]*PeerState),
		peerBitfields: make(map[[20]byte][]byte),
		infoHash:      infoHash,
		pieces:        pieceMgr,
		localPeerID:   localPeerId,
	}
}

func (s *Swarm) AddPeer(p Peer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.peers[p.ID()]; exists {
		return fmt.Errorf("peer %s already exists", p.ID())
	}
	s.peers[p.ID()] = p
	s.peerStates[p.ID()] = NewPeerState()
	return nil
}

func (s *Swarm) RemovePeer(id [20]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, exists := s.peers[id]; exists {
		_ = p.Close()
		delete(s.peers, id)
		delete(s.peerStates, id)
		delete(s.peerBitfields, id)
		fmt.Printf("peer %s removed from swarm\n", id)
	}
}

func (s *Swarm) GetPeer(id [20]byte) (Peer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, exists := s.peers[id]
	return p, exists
}

func (s *Swarm) OnPeer(p Peer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.peers[p.ID()]; exists {
		fmt.Printf("duplicate peer %s, closing\n", p.ID())
		_ = p.Close()
		return fmt.Errorf("peer %s already exists", p.ID())
	}

	fmt.Printf("peer %x joined torrent %x\n", p.ID(), s.infoHash)
	s.peers[p.ID()] = p
	s.peerStates[p.ID()] = NewPeerState()

	bitfield := s.pieces.Bitfield()
	msg := append([]byte{MsgBitfield}, bitfield...)
	if err := p.Send(msg); err != nil {
		fmt.Printf("failed to send bitfield to %s: %v\n", p.ID(), err)
		_ = p.Close()
		delete(s.peers, p.ID())
		delete(s.peerStates, p.ID())
		return fmt.Errorf("failed to send bitfield to %s: %v", p.ID(), err)
	}

	return nil
}

func (s *Swarm) Peers() []Peer {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers := make([]Peer, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	return peers
}

func (s *Swarm) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, p := range s.peers {
		_ = p.Close()
		delete(s.peers, id)
	}
}

func (s *Swarm) MissingPieces(bitfield []byte) []int {
	return s.pieces.MissingPieces(bitfield)
}

func (s *Swarm) GetPiece(idx int) ([]byte, bool) {
	return s.pieces.GetPiece(idx)
}

func (s *Swarm) NumPieces() int {
	return s.pieces.NumPieces()
}

func (s *Swarm) AddPiece(idx int, data []byte) error {
	if err := s.pieces.AddPiece(idx, data); err != nil {
		return err
	}
	return nil
}

func (s *Swarm) AllPiecesReceived() bool {
	received := s.pieces.ReceivedCount()
	total := s.pieces.NumPieces()
	fmt.Printf("pieces received: %d / %d\n", received, total)
	return s.pieces.AllPiecesReceived()
}

func (s *Swarm) Bitfield() []byte {
	return s.pieces.Bitfield()
}

func (s *Swarm) MissingPiecesCount() int {
	return s.pieces.NumPieces() - s.pieces.ReceivedCount()
}

func (s *Swarm) VerifyPiece(idx int, data []byte) bool {
	return s.pieces.VerifyPiece(idx, data)
}

func (s *Swarm) SetPieceHashes(hashes []byte) {
	s.pieces.SetPieceHashes(hashes)
}

func (s *Swarm) GetPeerState(id [20]byte) (*PeerState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.peerStates[id]
	return state, exists
}

func (s *Swarm) SetPeerInterested(id [20]byte, interested bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, exists := s.peerStates[id]; exists {
		state.SetPeerInterested(interested)
	}
}

func (s *Swarm) SetPeerChoking(id [20]byte, choking bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, exists := s.peerStates[id]; exists {
		state.SetPeerChoking(choking)
	}
}

func (s *Swarm) RecordUpload(id [20]byte, bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, exists := s.peerStates[id]; exists {
		state.AddUploaded(bytes)
	}
}

func (s *Swarm) RecordDownload(id [20]byte, bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, exists := s.peerStates[id]; exists {
		state.AddDownloaded(bytes)
	}
}

type peerRanking struct {
	id   [20]byte
	rate float64
}

func (s *Swarm) RunUnchokeAlgorithm() []UnchokeAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.unchokeRound++

	for _, state := range s.peerStates {
		state.UpdateRates()
	}

	var interested []peerRanking
	for id, state := range s.peerStates {
		if state.IsPeerInterested() {
			interested = append(interested, peerRanking{
				id:   id,
				rate: state.DownloadRate(),
			})
		}
	}

	sort.Slice(interested, func(i, j int) bool {
		return interested[i].rate > interested[j].rate
	})

	toUnchoke := make(map[[20]byte]bool)

	count := 0
	for _, pr := range interested {
		if count >= MaxUnchokedPeers-1 {
			break
		}
		toUnchoke[pr.id] = true
		count++
	}

	if s.unchokeRound%OptimisticUnchokeInterval == 0 || s.optimisticPeer == [20]byte{} {
		var chokedInterested [][20]byte
		for id, state := range s.peerStates {
			if state.IsPeerInterested() && !toUnchoke[id] {
				chokedInterested = append(chokedInterested, id)
			}
		}
		if len(chokedInterested) > 0 {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chokedInterested))))
			s.optimisticPeer = chokedInterested[n.Int64()]
		}
	}

	if s.optimisticPeer != ([20]byte{}) {
		if _, exists := s.peerStates[s.optimisticPeer]; exists {
			toUnchoke[s.optimisticPeer] = true
		}
	}

	var actions []UnchokeAction

	for id, state := range s.peerStates {
		shouldUnchoke := toUnchoke[id]
		currentlyChoking := state.IsAmChoking()

		if shouldUnchoke && currentlyChoking {
			state.SetAmChoking(false)
			actions = append(actions, UnchokeAction{PeerID: id, Unchoke: true})
		} else if !shouldUnchoke && !currentlyChoking {
			state.SetAmChoking(true)
			actions = append(actions, UnchokeAction{PeerID: id, Unchoke: false})
		}
	}

	return actions
}

type UnchokeAction struct {
	PeerID  [20]byte
	Unchoke bool
}

func (s *Swarm) IsChoking(id [20]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, exists := s.peerStates[id]; exists {
		return state.IsAmChoking()
	}
	return true
}

func (s *Swarm) UpdatePeerBitfield(id [20]byte, bitfield []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bf := make([]byte, len(bitfield))
	copy(bf, bitfield)
	s.peerBitfields[id] = bf
}

func (s *Swarm) SetPeerHasPiece(id [20]byte, pieceIdx int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bf, exists := s.peerBitfields[id]
	if !exists {
		size := (s.pieces.NumPieces() + 7) / 8
		bf = make([]byte, size)
		s.peerBitfields[id] = bf
	}

	byteIndex := pieceIdx / 8
	bitIndex := 7 - (pieceIdx % 8)
	if byteIndex < len(bf) {
		bf[byteIndex] |= 1 << bitIndex
	}
}

type pieceRarity struct {
	index int
	count int
}

func (s *Swarm) GetRarestMissingPieces(peerBitfield []byte) []int {
	s.mu.Lock()
	defer s.mu.Unlock()

	numPieces := s.pieces.NumPieces()
	availability := make([]int, numPieces)

	for _, bf := range s.peerBitfields {
		for i := 0; i < numPieces; i++ {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			if byteIndex < len(bf) && (bf[byteIndex]&(1<<bitIndex)) != 0 {
				availability[i]++
			}
		}
	}

	var candidates []pieceRarity
	for i := 0; i < numPieces; i++ {
		if _, exists := s.pieces.pieces[i]; exists {
			continue
		}

		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		if byteIndex >= len(peerBitfield) {
			continue
		}

		peerHas := (peerBitfield[byteIndex] & (1 << bitIndex)) != 0
		if peerHas {
			candidates = append(candidates, pieceRarity{
				index: i,
				count: availability[i],
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count == candidates[j].count {
			n, _ := rand.Int(rand.Reader, big.NewInt(2))
			return n.Int64() == 0
		}
		return candidates[i].count < candidates[j].count
	})

	result := make([]int, len(candidates))
	for i, c := range candidates {
		result[i] = c.index
	}
	return result
}
