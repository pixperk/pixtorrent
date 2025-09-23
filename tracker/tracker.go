package tracker

import "time"

type Peer struct {
	ID         string
	IP         string
	Port       int
	Uploaded   int64
	Downloaded int64
	Left       int64
	LastSeen   time.Time
}

type TorrentInfo struct {
	InfoHash  [20]byte
	Seeders   []string
	Leechers  []string
	Completed int
}

type Storage interface {
	// Peer management
	AddPeer(infoHash [20]byte, peer *Peer) error
	RemovePeer(infoHash [20]byte, peerID string) error
	GetPeer(peerID string) (*Peer, error)
	GetPeers(infoHash [20]byte, maxPeers int) ([]*Peer, error)
	GetSeeders(infoHash [20]byte, maxPeers int) ([]*Peer, error)
	GetLeechers(infoHash [20]byte, maxPeers int) ([]*Peer, error)
	UpdatePeerLastSeen(peerID string) error

	// Torrent management
	GetTorrentStats(infoHash [20]byte) (*TorrentInfo, error)
	IncrementCompleted(infoHash [20]byte) error
	GetActiveTorrents() ([]string, error)

	// Maintenance
	CleanupExpiredPeers() error
	Close() error
}
