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
		ListenAddr: "127.0.0.1:6881",
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}
	tcpOpts2 := p2p.TCPTransportOpts{
		ListenAddr: "127.0.0.1:6882",
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}

	server1 := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		Transport:        p2p.NewTCPTransport(tcpOpts1),
		TCPTransportOpts: tcpOpts1,
		TrackerUrl:       trackerUrl,
	})

	server2 := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		Transport:        p2p.NewTCPTransport(tcpOpts2),
		TCPTransportOpts: tcpOpts2,
		TrackerUrl:       trackerUrl,
	})

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

	// Give them time to bootstrap
	time.Sleep(2 * time.Second)

	// Wait until server1 sees a peer
	for len(server1.Swarm().Peers()) == 0 {
		time.Sleep(200 * time.Millisecond)
	}

	log.Println("Peers connected! Sending messages…")
	//debug swarm log
	var srv2PeerID [20]byte
	for _, peer := range server1.Swarm().Peers() {
		srv2PeerID = peer.ID()
		log.Printf("Server1 connected to peer: %x", peer.ID())

	}
	for _, peer := range server2.Swarm().Peers() {
		log.Printf("Server2 connected to peer: %x", peer.ID())
	}

	// Send messages both ways
	_ = server1.RequestPiece(0)
	_ = server2.RequestPiece(1)
	if err := server1.SendPiece(6, srv2PeerID); err != nil {
		panic("SendPiece error:	 " + err.Error())
	}

	select {}
}
