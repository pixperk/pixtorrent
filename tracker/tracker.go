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
	GetTorrent(infoHash [20]byte) (*TorrentInfo, error)
	UpdateTorrent(infoHash [20]byte, torrent *TorrentInfo) error
	AddPeer(infoHash [20]byte, peer *Peer, event string) error
	RemovePeer(infoHash [20]byte, peerID string) error
	UpdatePeer(infoHash [20]byte, peer *Peer) error
	GetPeers(infoHash [20]byte, numWant int, compact bool) ([]*Peer, error)
}
