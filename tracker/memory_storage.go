package tracker

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type MemoryStorage struct {
	mu        sync.RWMutex
	peers     map[string]*Peer
	torrents  map[string]map[string]struct{}
	completed map[string]int
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		peers:     make(map[string]*Peer),
		torrents:  make(map[string]map[string]struct{}),
		completed: make(map[string]int),
	}
}

func (m *MemoryStorage) Close() error {
	return nil
}

func (m *MemoryStorage) AddPeer(infoHash [20]byte, peer *Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := hex.EncodeToString(infoHash[:])

	m.peers[peer.ID] = peer

	if m.torrents[hashKey] == nil {
		m.torrents[hashKey] = make(map[string]struct{})
	}
	m.torrents[hashKey][peer.ID] = struct{}{}

	return nil
}

func (m *MemoryStorage) RemovePeer(infoHash [20]byte, peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := hex.EncodeToString(infoHash[:])

	delete(m.peers, peerID)
	if m.torrents[hashKey] != nil {
		delete(m.torrents[hashKey], peerID)
	}

	return nil
}

func (m *MemoryStorage) GetPeer(peerID string) (*Peer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peer, exists := m.peers[peerID]
	if !exists {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}
	return peer, nil
}

func (m *MemoryStorage) GetPeers(infoHash [20]byte, maxPeers int) ([]*Peer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashKey := hex.EncodeToString(infoHash[:])
	peerIDs := m.torrents[hashKey]
	if peerIDs == nil {
		return []*Peer{}, nil
	}

	peers := make([]*Peer, 0, len(peerIDs))
	count := 0
	for peerID := range peerIDs {
		if count >= maxPeers {
			break
		}
		if peer, exists := m.peers[peerID]; exists {
			peers = append(peers, peer)
			count++
		}
	}

	return peers, nil
}

func (m *MemoryStorage) GetSeeders(infoHash [20]byte, maxPeers int) ([]*Peer, error) {
	peers, err := m.GetPeers(infoHash, maxPeers*2)
	if err != nil {
		return nil, err
	}

	seeders := make([]*Peer, 0)
	for _, peer := range peers {
		if peer.Left == 0 && len(seeders) < maxPeers {
			seeders = append(seeders, peer)
		}
	}
	return seeders, nil
}

func (m *MemoryStorage) GetLeechers(infoHash [20]byte, maxPeers int) ([]*Peer, error) {
	peers, err := m.GetPeers(infoHash, maxPeers*2)
	if err != nil {
		return nil, err
	}

	leechers := make([]*Peer, 0)
	for _, peer := range peers {
		if peer.Left > 0 && len(leechers) < maxPeers {
			leechers = append(leechers, peer)
		}
	}
	return leechers, nil
}

func (m *MemoryStorage) GetTorrentStats(infoHash [20]byte) (*TorrentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashKey := hex.EncodeToString(infoHash[:])
	peerIDs := m.torrents[hashKey]

	seeders := make([]string, 0)
	leechers := make([]string, 0)

	if peerIDs != nil {
		for peerID := range peerIDs {
			if peer, exists := m.peers[peerID]; exists {
				if peer.Left == 0 {
					seeders = append(seeders, peerID)
				} else {
					leechers = append(leechers, peerID)
				}
			}
		}
	}

	return &TorrentInfo{
		InfoHash:  infoHash,
		Seeders:   seeders,
		Leechers:  leechers,
		Completed: m.completed[hashKey],
	}, nil
}

func (m *MemoryStorage) IncrementCompleted(infoHash [20]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := hex.EncodeToString(infoHash[:])
	m.completed[hashKey]++
	return nil
}

func (m *MemoryStorage) UpdatePeerLastSeen(peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if peer, exists := m.peers[peerID]; exists {
		peer.LastSeen = time.Now()
	}
	return nil
}

func (m *MemoryStorage) CleanupExpiredPeers() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-30 * time.Minute)

	for peerID, peer := range m.peers {
		if peer.LastSeen.Before(cutoff) {
			delete(m.peers, peerID)
			for _, peerSet := range m.torrents {
				delete(peerSet, peerID)
			}
		}
	}

	return nil
}

func (m *MemoryStorage) GetActiveTorrents() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	torrents := make([]string, 0, len(m.torrents))
	for hash := range m.torrents {
		torrents = append(torrents, hash)
	}
	return torrents, nil
}
