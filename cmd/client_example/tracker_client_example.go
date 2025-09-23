package main

import (
	"fmt"
	"log"

	"github.com/pixperk/pixtorrent/client"
	"github.com/pixperk/pixtorrent/meta"
)

func main() {
	torrentData := []byte("d8:announce21:http://localhost:80804:infod6:lengthi1024e4:name8:test.txt12:piece lengthi32768e6:pieces20:12345678901234567890ee")

	torrent, err := meta.ParseTorrent(torrentData)
	if err != nil {
		log.Fatal("Failed to parse torrent:", err)
	}

	infoHash, err := torrent.InfoHash()
	if err != nil {
		log.Fatal("Failed to get info hash:", err)
	}

	fmt.Printf("Torrent: %s\n", torrent.Info.Name)
	fmt.Printf("Tracker: %s\n", torrent.Announce)
	fmt.Printf("Info Hash: %x\n", infoHash)

	fmt.Println("\nTesting multiple peers")
	testMultiplePeers(torrent.Announce, infoHash, torrent.Info.Length)
}

func testMultiplePeers(trackerURL string, infoHash [20]byte, fileSize int64) {
	peers := []struct {
		id   string
		port int
		name string
	}{
		{"-PC0001-123456789012", 6881, "Peer A"},
		{"-PC0002-abcdefghijkl", 6882, "Peer B"},
		{"-PC0003-mnopqrstuvwx", 6883, "Peer C"},
	}

	clients := make([]*client.TrackerClient, len(peers))

	fmt.Println("\nStep 1: All peers announce started")
	for i, peer := range peers {
		clients[i] = client.NewTrackerClient(peer.id, peer.port)
		response, err := clients[i].Announce(trackerURL, infoHash, fileSize, "started")
		if err != nil {
			log.Printf("%s announce failed: %v", peer.name, err)
		} else {
			fmt.Printf("%s: Interval=%ds, Peers=%d\n", peer.name, response.Interval, len(response.Peers))
			for j, p := range response.Peers {
				fmt.Printf("  Peer %d: %s:%d (ID: %s)\n", j+1, p.IP, p.Port, p.PeerID)
			}
		}
	}

	fmt.Println("\nStep 2: Peer A downloads some data")
	clients[0].UpdateStats(512, 256)
	response, err := clients[0].Announce(trackerURL, infoHash, fileSize-256, "")
	if err != nil {
		log.Printf("Peer A update failed: %v", err)
	} else {
		fmt.Printf("Peer A: Updated stats, Peers=%d\n", len(response.Peers))
		for i, p := range response.Peers {
			fmt.Printf("  Peer %d: %s:%d (ID: %s)\n", i+1, p.IP, p.Port, p.PeerID)
		}
	}

	fmt.Println("\nStep 3: Peer B completes download")
	clients[1].UpdateStats(1024, fileSize)
	response, err = clients[1].Announce(trackerURL, infoHash, 0, "completed")
	if err != nil {
		log.Printf("Peer B completion failed: %v", err)
	} else {
		fmt.Printf("Peer B: Download completed! Peers=%d\n", len(response.Peers))
		for i, p := range response.Peers {
			fmt.Printf("  Peer %d: %s:%d (ID: %s)\n", i+1, p.IP, p.Port, p.PeerID)
		}
	}

	fmt.Println("\nStep 4: Peer C checks current peers")
	response, err = clients[2].Announce(trackerURL, infoHash, fileSize, "")
	if err != nil {
		log.Printf("Peer C update failed: %v", err)
	} else {
		fmt.Printf("Peer C sees %d other peers:\n", len(response.Peers))
		for i, p := range response.Peers {
			fmt.Printf("  Peer %d: %s:%d (ID: %s)\n", i+1, p.IP, p.Port, p.PeerID)
		}
	}

	fmt.Println("\nStep 5: All peers stop")
	for i, peer := range peers {
		_, err := clients[i].Announce(trackerURL, infoHash, 0, "stopped")
		if err != nil {
			log.Printf("%s stop failed: %v", peer.name, err)
		} else {
			fmt.Printf("%s: Stopped announcing\n", peer.name)
		}
	}
}
