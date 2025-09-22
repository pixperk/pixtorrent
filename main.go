package main

import (
	"fmt"
	"log"

	"github.com/pixperk/pixtorrent/meta"
)

func main() {
	fmt.Println("=== SIMPLE TORRENT ===")
	simpleData := []byte("d8:announce9:localhost4:infod6:lengthi1024e4:name8:test.txt12:piece lengthi32768e6:pieces20:12345678901234567890ee")
	processTorrent(simpleData)

	fmt.Println("\n=== LARGER TORRENT ===")
	largerData := []byte("d8:announce15:http://test.com4:infod6:lengthi2048576e4:name9:video.mp412:piece lengthi524288e6:pieces40:1234567890123456789012345678901234567890ee")
	processTorrent(largerData)

	fmt.Println("\n=== MULTI FILE TORRENT ===")
	multiData := []byte("d8:announce15:http://test.com4:infod5:filesld6:lengthi1000e4:pathl7:doc.pdfeed6:lengthi2000e4:pathl8:code.cppeed6:lengthi500e4:pathl6:READMEeee4:name7:project12:piece lengthi65536e6:pieces40:1234567890123456789012345678901234567890ee")
	processTorrent(multiData)
}

func processTorrent(data []byte) {
	torrent, err := meta.ParseTorrent(data)
	if err != nil {
		log.Fatal(err)
	}

	hash, err := torrent.InfoHash()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s\n", torrent.Info.Name)
	fmt.Printf("Tracker: %s\n", torrent.Announce)
	fmt.Printf("Info Hash: %x\n", hash)
	fmt.Printf("Piece Length: %d bytes\n", torrent.Info.PieceLength)
	fmt.Printf("Total Pieces: %d\n", len(torrent.Info.Pieces)/20)

	if len(torrent.AnnounceList) > 0 {
		fmt.Printf("Backup Trackers: %d tiers\n", len(torrent.AnnounceList))
		for i, tier := range torrent.AnnounceList {
			fmt.Printf("  Tier %d: %v\n", i+1, tier)
		}
	}

	if torrent.Info.Length > 0 {
		fmt.Printf("Type: Single File\n")
		fmt.Printf("Size: %d bytes (%.2f MB)\n", torrent.Info.Length, float64(torrent.Info.Length)/1024/1024)
	} else {
		fmt.Printf("Type: Multi File\n")
		fmt.Printf("Files: %d\n", len(torrent.Info.Files))
		var totalSize int64
		for i, file := range torrent.Info.Files {
			totalSize += file.Length
			fmt.Printf("  [%d] %v - %d bytes\n", i+1, file.Path, file.Length)
		}

	}

	if torrent.CreationDate > 0 {
		fmt.Printf("Created: %d\n", torrent.CreationDate)
	}
	if torrent.CreatedBy != "" {
		fmt.Printf("Created By: %s\n", torrent.CreatedBy)
	}
	if torrent.Comment != "" {
		fmt.Printf("Comment: %s\n", torrent.Comment)
	}
}
