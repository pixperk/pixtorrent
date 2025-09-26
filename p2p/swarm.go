package p2p

import (
	"fmt"
	"sync"
)

type Swarm struct {
	peers    map[[20]byte]Peer
	mu       sync.Mutex
	infoHash [20]byte
}

func NewSwarm(infoHash [20]byte) *Swarm {
	return &Swarm{
		peers:    make(map[[20]byte]Peer),
		infoHash: infoHash,
	}
}

func (s *Swarm) AddPeer(p Peer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.peers[p.ID()]; exists {
		return fmt.Errorf("peer %s already exists", p.ID())
	}
	s.peers[p.ID()] = p
	return nil
}

func (s *Swarm) RemovePeer(id [20]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, exists := s.peers[id]; exists {
		_ = p.Close()
		delete(s.peers, id)
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

	// Reject duplicate or self peers
	if _, exists := s.peers[p.ID()]; exists {
		fmt.Printf("duplicate peer %s, closing\n", p.ID())
		_ = p.Close()
		return fmt.Errorf("peer %s already exists", p.ID())
	}

	fmt.Printf("peer %s joined torrent %x\n", p.ID(), s.infoHash)
	s.peers[p.ID()] = p
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
