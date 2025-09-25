package p2p

import (
	"fmt"
	"sync"
)

type Swarm struct {
	peers    map[string]Peer
	mu       sync.Mutex
	infoHash [20]byte
}

func NewSwarm(infoHash [20]byte) *Swarm {
	return &Swarm{
		peers:    make(map[string]Peer),
		mu:       sync.Mutex{},
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

func (s *Swarm) OnPeer(p Peer) error {
	fmt.Printf("peer %s joined torrent %x\n", p.ID(), s.infoHash)
	s.AddPeer(p)
	//send initial message [bitfield, interested, etc]
	return nil
}
