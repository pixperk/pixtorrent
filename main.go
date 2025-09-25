package main

import (
	"log"

	"github.com/pixperk/pixtorrent/p2p"
	torrentserver "github.com/pixperk/pixtorrent/torrent_server"
)

func main() {
	infoHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	ports := []string{"127.0.0.1:6881", "127.0.0.1:6882", "127.0.0.1:6883"}
	servers := make([]*torrentserver.TorrentServer, 3)

	for i := 0; i < 3; i++ {
		tcpOpts := p2p.TCPTransportOpts{
			ListenAddr: ports[i],
			InfoHash:   infoHash,
			Handshake:  p2p.DefaultHandshakeFunc,
			Decoder:    &p2p.BinaryDecoder{},
		}

		tcpTransport := p2p.NewTCPTransport(tcpOpts)

		server := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
			Transport:        tcpTransport,
			TCPTransportOpts: tcpOpts,
			TrackerUrl:       "http://localhost:8080/announce",
		})

		servers[i] = server

		go func(s *torrentserver.TorrentServer) {
			if err := s.Start(); err != nil {
				log.Fatal(err)
			}
		}(server)
	}

	select {} // block forever
}
