package client

import (
	"testing"

	"github.com/pixperk/pixtorrent/meta"
)

func TestTrackerClientAnnounce(t *testing.T) {
	torrentData := []byte("d8:announce21:http://localhost:80804:infod6:lengthi1024e4:name8:test.txt12:piece lengthi32768e6:pieces20:12345678901234567890ee")
	torrent, err := meta.ParseTorrent(torrentData)
	if err != nil {
		t.Fatalf("parse torrent: %v", err)
	}
	infoHash, err := torrent.InfoHash()
	if err != nil {
		t.Fatalf("info hash: %v", err)
	}
	peerID := "-PC0001-123456789012"
	port := 6881
	c := NewTrackerClient(peerID, port)
	resp, err := c.Announce(torrent.Announce, infoHash, torrent.Info.Length, "started")
	if err != nil {
		t.Fatalf("announce: %v", err)
	}
	if resp.Interval <= 0 {
		t.Errorf("interval not positive")
	}
}

func TestTrackerClientMultiplePeers(t *testing.T) {
	torrentData := []byte("d8:announce21:http://localhost:80804:infod6:lengthi1024e4:name8:test.txt12:piece lengthi32768e6:pieces20:12345678901234567890ee")
	torrent, err := meta.ParseTorrent(torrentData)
	if err != nil {
		t.Fatalf("parse torrent: %v", err)
	}
	infoHash, err := torrent.InfoHash()
	if err != nil {
		t.Fatalf("info hash: %v", err)
	}
	peers := []struct {
		id   string
		port int
	}{
		{"-PC0001-123456789012", 6881},
		{"-PC0002-abcdefghijkl", 6882},
		{"-PC0003-mnopqrstuvwx", 6883},
	}
	clients := make([]*TrackerClient, len(peers))
	for i, p := range peers {
		clients[i] = NewTrackerClient(p.id, p.port)
		_, err := clients[i].Announce(torrent.Announce, infoHash, torrent.Info.Length, "started")
		if err != nil {
			t.Fatalf("peer %d announce: %v", i, err)
		}
	}
	resp, err := clients[0].Announce(torrent.Announce, infoHash, torrent.Info.Length, "")
	if err != nil {
		t.Fatalf("peer 0 update: %v", err)
	}
	if len(resp.Peers) == 0 {
		t.Errorf("expected peers, got 0")
	}
}

func TestTrackerClientScrape(t *testing.T) {
	torrentData := []byte("d8:announce21:http://localhost:80804:infod6:lengthi1024e4:name8:test.txt12:piece lengthi32768e6:pieces20:12345678901234567890ee")
	torrent, err := meta.ParseTorrent(torrentData)
	if err != nil {
		t.Fatalf("parse torrent: %v", err)
	}
	infoHash, err := torrent.InfoHash()
	if err != nil {
		t.Fatalf("info hash: %v", err)
	}
	peerID := "-PC0001-123456789012"
	port := 6881
	c := NewTrackerClient(peerID, port)
	_, err = c.Announce(torrent.Announce, infoHash, torrent.Info.Length, "started")
	if err != nil {
		t.Fatalf("announce: %v", err)
	}
	// Scrape request
	resp, err := c.Scrape(torrent.Announce, infoHash)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if resp.Complete < 0 || resp.Incomplete < 0 {
		t.Errorf("invalid scrape stats")
	}
}
