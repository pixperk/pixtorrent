package main

import (
	"fmt"
	"log"

	"github.com/pixperk/pixtorrent/client"
	"github.com/pixperk/pixtorrent/meta"
)

func main() {
	// Example: Using the tracker client

	// 1. Parse a torrent file to get info hash and tracker URL
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

	// 2. Create tracker client
	peerID := "-PC0001-123456789012" // Your peer ID
	port := 6881                     // Your listening port

	client := client.NewTrackerClient(peerID, port)

	// 3. Make announce requests
	fmt.Println("\n=== ANNOUNCE: STARTED ===")
	response, err := client.Announce(torrent.Announce, infoHash, torrent.Info.Length, "started")
	if err != nil {
		log.Printf("Announce failed: %v", err)
	} else {
		fmt.Printf("Interval: %d seconds\n", response.Interval)
		fmt.Printf("Peers found: %d\n", len(response.Peers))
		for i, peer := range response.Peers {
			fmt.Printf("  Peer %d: %s:%d (ID: %s)\n", i+1, peer.IP, peer.Port, peer.PeerID)
		}
	}

	// 4. Simulate download progress
	client.UpdateStats(1024, 512) // uploaded 1KB, downloaded 512B

	fmt.Println("\n=== ANNOUNCE: UPDATE ===")
	response, err = client.Announce(torrent.Announce, infoHash, torrent.Info.Length-512, "")
	if err != nil {
		log.Printf("Announce failed: %v", err)
	} else {
		fmt.Printf("Updated stats sent. Peers: %d\n", len(response.Peers))
	}

	// 5. Announce completion
	client.UpdateStats(2048, torrent.Info.Length) // Download complete

	fmt.Println("\n=== ANNOUNCE: COMPLETED ===")
	response, err = client.Announce(torrent.Announce, infoHash, 0, "completed")
	if err != nil {
		log.Printf("Announce failed: %v", err)
	} else {
		fmt.Printf("Download completed! Final peer count: %d\n", len(response.Peers))
	}

	// 6. Stop announcing
	fmt.Println("\n=== ANNOUNCE: STOPPED ===")
	response, err = client.Announce(torrent.Announce, infoHash, 0, "stopped")
	if err != nil {
		log.Printf("Announce failed: %v", err)
	} else {
		fmt.Printf("Stopped announcing.\n")
	}
}
