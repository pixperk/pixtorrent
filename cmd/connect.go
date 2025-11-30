package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pixperk/pixtorrent/p2p"
	torrentserver "github.com/pixperk/pixtorrent/torrent_server"
	"github.com/spf13/cobra"
)

var (
	connectInfoHash string
	connectPort     string
	connectTracker  string
	connectPieces   int
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a swarm without downloading",
	Long:  `Join a torrent swarm as a peer. Useful for observing swarm activity.`,
	RunE:  runConnect,
}

func init() {
	connectCmd.Flags().StringVarP(&connectInfoHash, "hash", "i", "", "Info hash of the torrent (40 hex chars, required)")
	connectCmd.Flags().StringVarP(&connectPort, "port", "p", "0", "Port to listen on (0 for random)")
	connectCmd.Flags().StringVarP(&connectTracker, "tracker", "t", "http://localhost:8080", "Tracker URL")
	connectCmd.Flags().IntVarP(&connectPieces, "pieces", "n", 1, "Number of pieces in torrent")

	connectCmd.MarkFlagRequired("hash")
	rootCmd.AddCommand(connectCmd)
}

func runConnect(cmd *cobra.Command, args []string) error {
	if len(connectInfoHash) != 40 {
		return fmt.Errorf("info hash must be 40 hex characters")
	}

	hashBytes, err := hex.DecodeString(connectInfoHash)
	if err != nil {
		return fmt.Errorf("invalid info hash: %w", err)
	}

	var infoHash [20]byte
	copy(infoHash[:], hashBytes)

	// Create piece manager with no pieces (we're just connecting)
	pm := p2p.NewPieceManager(connectPieces)

	listenAddr := fmt.Sprintf("0.0.0.0:%s", connectPort)

	tcpOpts := p2p.TCPTransportOpts{
		ListenAddr: listenAddr,
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}

	server := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		TCPTransportOpts: tcpOpts,
		TrackerUrl:       connectTracker,
		RootDir:          "downloads",
		FileFormat:       "bin",
	}, pm)

	PrintLogoSmall()
	PrintHeader("CONNECTED")

	PrintSection("Swarm")
	PrintKeyValueHighlight("InfoHash", connectInfoHash)
	PrintKeyValue("Pieces", fmt.Sprintf("%d", connectPieces))

	PrintSection("Network")
	PrintKeyValue("Tracker", connectTracker)
	PrintStatus("Mode", "observer", Cyan)

	PrintDivider()
	PrintInfo("Joined swarm, listening for peers...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nLeaving swarm...")
		server.Stop()
		os.Exit(0)
	}()

	return server.Start()
}
