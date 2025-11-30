package p2p

import (
	"sync"
	"time"
)

type PeerState struct {
	mu sync.RWMutex

	AmChoking      bool
	AmInterested   bool
	PeerChoking    bool
	PeerInterested bool

	Uploaded      int64
	Downloaded    int64
	uploadRate    float64
	downloadRate  float64
	lastStatTime  time.Time
	lastUploaded  int64
	lastDownloaded int64
}

func NewPeerState() *PeerState {
	return &PeerState{
		AmChoking:    true,
		PeerChoking:  true,
		lastStatTime: time.Now(),
	}
}

func (ps *PeerState) SetAmChoking(v bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.AmChoking = v
}

func (ps *PeerState) SetAmInterested(v bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.AmInterested = v
}

func (ps *PeerState) SetPeerChoking(v bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.PeerChoking = v
}

func (ps *PeerState) SetPeerInterested(v bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.PeerInterested = v
}

func (ps *PeerState) AddUploaded(n int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.Uploaded += n
}

func (ps *PeerState) AddDownloaded(n int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.Downloaded += n
}

func (ps *PeerState) UpdateRates() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(ps.lastStatTime).Seconds()
	if elapsed < 1 {
		return
	}

	ps.uploadRate = float64(ps.Uploaded-ps.lastUploaded) / elapsed
	ps.downloadRate = float64(ps.Downloaded-ps.lastDownloaded) / elapsed

	ps.lastUploaded = ps.Uploaded
	ps.lastDownloaded = ps.Downloaded
	ps.lastStatTime = now
}

func (ps *PeerState) UploadRate() float64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.uploadRate
}

func (ps *PeerState) DownloadRate() float64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.downloadRate
}

func (ps *PeerState) IsAmChoking() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.AmChoking
}

func (ps *PeerState) IsPeerChoking() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.PeerChoking
}

func (ps *PeerState) IsPeerInterested() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.PeerInterested
}

func (ps *PeerState) IsAmInterested() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.AmInterested
}
