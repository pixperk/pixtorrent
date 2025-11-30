package main

import (
	"log"
	"math/rand"
	"os" // <-- needed to read file

	"github.com/pixperk/pixtorrent/p2p"
	torrentserver "github.com/pixperk/pixtorrent/torrent_server"
)

func main() {
	trackerUrl := "http://localhost:8080"
	fileFormat := "png"
	infoHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	// Fake text data
	//data1 := []byte("This is a longer fake file buffer to test partial piece distribution across peers!!!")

	// Real image file
	data2, err := os.ReadFile("image.png") // make sure image.png is in the same dir
	if err != nil {
		log.Fatalf("failed to read image: %v", err)
	}

	data := data2

	pieceSize := 10 * 1024
	numPieces := (len(data) + pieceSize - 1) / pieceSize

	pm1 := p2p.NewPieceManager(numPieces)
	pm2 := p2p.NewPieceManager(numPieces)
	pm3 := p2p.NewPieceManager(numPieces)
	pms := []*p2p.PieceManager{pm1, pm2, pm3}

	// Seed pieces randomly
	for i := 0; i < numPieces; i++ {
		start := i * pieceSize
		end := start + pieceSize
		if end > len(data) {
			end = len(data)
		}
		peerIndex := rand.Intn(len(pms))
		pms[peerIndex].AddPiece(i, data[start:end])
		log.Printf("Assigned piece %d to peer %d (%d bytes)\n", i, peerIndex+1, end-start)
	}

	// TCP transports
	tcpOpts := []p2p.TCPTransportOpts{
		{ListenAddr: "127.0.0.1:3000", InfoHash: infoHash, Handshake: p2p.DefaultHandshakeFunc, Decoder: &p2p.BinaryDecoder{}},
		{ListenAddr: "127.0.0.1:4000", InfoHash: infoHash, Handshake: p2p.DefaultHandshakeFunc, Decoder: &p2p.BinaryDecoder{}},
		{ListenAddr: "127.0.0.1:5000", InfoHash: infoHash, Handshake: p2p.DefaultHandshakeFunc, Decoder: &p2p.BinaryDecoder{}},
	}

	servers := []*torrentserver.TorrentServer{
		torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
			Transport: p2p.NewTCPTransport(tcpOpts[0]), TCPTransportOpts: tcpOpts[0], TrackerUrl: trackerUrl, RootDir: "server1_data",
			FileFormat: fileFormat,
		}, pm1),
		torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
			Transport: p2p.NewTCPTransport(tcpOpts[1]), TCPTransportOpts: tcpOpts[1], TrackerUrl: trackerUrl, RootDir: "server2_data",
			FileFormat: fileFormat,
		}, pm2),
		torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
			Transport: p2p.NewTCPTransport(tcpOpts[2]), TCPTransportOpts: tcpOpts[2], TrackerUrl: trackerUrl, RootDir: "server3_data",
			FileFormat: fileFormat,
		}, pm3),
	}

	// Start servers
	for _, srv := range servers {
		go func(s *torrentserver.TorrentServer) {
			if err := s.Start(); err != nil {
				log.Fatal(err)
			}
		}(srv)
	}

	select {}
}
