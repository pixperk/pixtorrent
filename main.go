package main

import (
	"log"
	"time"

	"github.com/pixperk/pixtorrent/p2p"
	torrentserver "github.com/pixperk/pixtorrent/torrent_server"
)

func main() {

	trackerUrl := "http://localhost:8080"
	infoHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	// Create TCP transports
	tcpOpts1 := p2p.TCPTransportOpts{
		ListenAddr: "127.0.0.1:3000",
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}
	tcpOpts2 := p2p.TCPTransportOpts{
		ListenAddr: "127.0.0.1:4000",
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}
	/* 	tcpOpts3 := p2p.TCPTransportOpts{
		ListenAddr: "127.0.0.1:5000",
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	} */

	data := []byte("hello world")
	pieceSize := 5
	numPieces := (len(data) + pieceSize - 1) / pieceSize

	pm1 := p2p.NewPieceManager(numPieces)
	pm2 := p2p.NewPieceManager(numPieces)
	/* 	pm3 := p2p.NewPieceManager(numPieces) */

	// assign some pieces
	pm1.AddPiece(0, data[0:5]) // "hello"
	pm2.AddPiece(1, data[5:])  // " world"
	/* pm3.AddPiece(2, data[10:])  // "d" */

	server1 := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		Transport:        p2p.NewTCPTransport(tcpOpts1),
		TCPTransportOpts: tcpOpts1,
		TrackerUrl:       trackerUrl,
	}, pm1)

	server2 := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		Transport:        p2p.NewTCPTransport(tcpOpts2),
		TCPTransportOpts: tcpOpts2,
		TrackerUrl:       trackerUrl,
	}, pm2)

	/* 	server3 := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		Transport:        p2p.NewTCPTransport(tcpOpts3),
		TCPTransportOpts: tcpOpts3,
		TrackerUrl:       trackerUrl,
	}, pm3) */

	go func() {
		if err := server1.Start(); err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		if err := server2.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	/* go func() {
		if err := server3.Start(); err != nil {
			log.Fatal(err)
		}
	}()
	*/

	// Give them time to bootstrap
	time.Sleep(2 * time.Second)

	// Wait until server1 sees a peer
	for len(server1.Swarm().Peers()) == 0 {
		time.Sleep(200 * time.Millisecond)
	}

	select {}
}
