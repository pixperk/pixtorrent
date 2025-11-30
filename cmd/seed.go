package cmd

import (
	"crypto/sha1"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pixperk/pixtorrent/p2p"
	torrentserver "github.com/pixperk/pixtorrent/torrent_server"
	"github.com/spf13/cobra"
)

var (
	seedFile      string
	seedPort      string
	seedTracker   string
	seedPieceSize int
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed a file to the network",
	Long:  `Start seeding a file, making it available for other peers to download.`,
	RunE:  runSeed,
}

func init() {
	seedCmd.Flags().StringVarP(&seedFile, "file", "f", "", "File to seed (required)")
	seedCmd.Flags().StringVarP(&seedPort, "port", "p", "0", "Port to listen on (0 for random)")
	seedCmd.Flags().StringVarP(&seedTracker, "tracker", "t", "http://localhost:8080", "Tracker URL")
	seedCmd.Flags().IntVarP(&seedPieceSize, "piece-size", "s", 16384, "Piece size in bytes")

	seedCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(seedCmd)
}

func runSeed(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	infoHash := sha1.Sum(data)
	numPieces := (len(data) + seedPieceSize - 1) / seedPieceSize

	pieceHashes := make([]byte, numPieces*20)
	for i := 0; i < numPieces; i++ {
		start := i * seedPieceSize
		end := start + seedPieceSize
		if end > len(data) {
			end = len(data)
		}
		hash := sha1.Sum(data[start:end])
		copy(pieceHashes[i*20:(i+1)*20], hash[:])
	}

	pm := p2p.NewPieceManagerWithHashes(numPieces, pieceHashes)

	for i := 0; i < numPieces; i++ {
		start := i * seedPieceSize
		end := start + seedPieceSize
		if end > len(data) {
			end = len(data)
		}
		pm.AddPiece(i, data[start:end])
	}

	listenAddr := fmt.Sprintf("0.0.0.0:%s", seedPort)
	ext := filepath.Ext(seedFile)
	if len(ext) > 0 {
		ext = ext[1:]
	}

	tcpOpts := p2p.TCPTransportOpts{
		ListenAddr: listenAddr,
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}

	server := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		TCPTransportOpts: tcpOpts,
		TrackerUrl:       seedTracker,
		RootDir:          "downloads",
		FileFormat:       ext,
	}, pm)

	pieceHashHex := fmt.Sprintf("%x", pieceHashes)

	fmt.Println()
	fmt.Println("  pixTorrent Seeder")
	fmt.Println("  -----------------")
	fmt.Printf("  File:       %s\n", seedFile)
	fmt.Printf("  Size:       %d bytes\n", len(data))
	fmt.Printf("  Pieces:     %d x %d bytes\n", numPieces, seedPieceSize)
	fmt.Printf("  InfoHash:   %x\n", infoHash)
	fmt.Printf("  Tracker:    %s\n", seedTracker)
	fmt.Println()
	fmt.Println("  To download, run:")
	fmt.Printf("  pixtorrent download -i %x -n %d -f %s -t %s -H %s\n", infoHash, numPieces, ext, seedTracker, pieceHashHex)
	fmt.Println()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		server.Stop()
		os.Exit(0)
	}()

	return server.Start()
}
