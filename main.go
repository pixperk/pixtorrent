package main

import (
	"log"
	"time"

	"github.com/pixperk/pixtorrent/p2p"
	torrentserver "github.com/pixperk/pixtorrent/torrent_server"
)

func main() {
	infoHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	ports := []string{"127.0.0.1:6881", "127.0.0.1:6882", "127.0.0.1:6883"}
	servers := make([]*torrentserver.TorrentServer, 3)

	readyChans := make([]chan struct{}, len(ports))

	for i := 0; i < 3; i++ {
		ready := make(chan struct{})
		readyChans[i] = ready

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

		go func(s *torrentserver.TorrentServer, ready chan struct{}) {
			if err := s.Start(); err != nil {
				log.Fatal(err)
			}
			close(ready) // signal that server is ready
		}(server, ready)
	}

	// wait for all servers to be ready
	for _, ch := range readyChans {
		<-ch
	}

	log.Println("All servers are ready. Sending messages...")

	// give a tiny delay to let peers connect and swarm populate
	time.Sleep(1 * time.Second)

	select {} // block forever
}
