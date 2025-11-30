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
	downloadInfoHash   string
	downloadPort       string
	downloadTracker    string
	downloadOutput     string
	downloadPieces     int
	downloadFormat     string
	downloadPieceHash  string
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a file from the network",
	Long:  `Connect to peers and download a file by its info hash.`,
	RunE:  runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&downloadInfoHash, "hash", "i", "", "Info hash of the file (40 hex chars, required)")
	downloadCmd.Flags().StringVarP(&downloadPort, "port", "p", "0", "Port to listen on (0 for random)")
	downloadCmd.Flags().StringVarP(&downloadTracker, "tracker", "t", "http://localhost:8080", "Tracker URL")
	downloadCmd.Flags().StringVarP(&downloadOutput, "output", "o", "downloads", "Output directory")
	downloadCmd.Flags().IntVarP(&downloadPieces, "pieces", "n", 1, "Expected number of pieces")
	downloadCmd.Flags().StringVarP(&downloadFormat, "format", "f", "bin", "Output file format/extension")
	downloadCmd.Flags().StringVarP(&downloadPieceHash, "piece-hashes", "H", "", "Piece hashes (hex string, 40 chars per piece)")

	downloadCmd.MarkFlagRequired("hash")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	if len(downloadInfoHash) != 40 {
		return fmt.Errorf("info hash must be 40 hex characters")
	}

	hashBytes, err := hex.DecodeString(downloadInfoHash)
	if err != nil {
		return fmt.Errorf("invalid info hash: %w", err)
	}

	var infoHash [20]byte
	copy(infoHash[:], hashBytes)

	var pieceHashes []byte
	if downloadPieceHash != "" {
		pieceHashes, err = hex.DecodeString(downloadPieceHash)
		if err != nil {
			return fmt.Errorf("invalid piece hashes: %w", err)
		}
	}

	var pm *p2p.PieceManager
	if len(pieceHashes) > 0 {
		pm = p2p.NewPieceManagerWithHashes(downloadPieces, pieceHashes)
	} else {
		pm = p2p.NewPieceManager(downloadPieces)
	}

	listenAddr := fmt.Sprintf("0.0.0.0:%s", downloadPort)

	tcpOpts := p2p.TCPTransportOpts{
		ListenAddr: listenAddr,
		InfoHash:   infoHash,
		Handshake:  p2p.DefaultHandshakeFunc,
		Decoder:    &p2p.BinaryDecoder{},
	}

	server := torrentserver.NewTorrentServer(torrentserver.TorrentServerOpts{
		TCPTransportOpts: tcpOpts,
		TrackerUrl:       downloadTracker,
		RootDir:          downloadOutput,
		FileFormat:       downloadFormat,
	}, pm)

	PrintLogoSmall()
	PrintHeader("DOWNLOADING")

	PrintSection("Target")
	PrintKeyValueHighlight("InfoHash", downloadInfoHash)
	PrintKeyValue("Pieces", fmt.Sprintf("%d", downloadPieces))

	PrintSection("Output")
	PrintKeyValue("Directory", downloadOutput+"/")
	PrintKeyValue("Format", "."+downloadFormat)

	PrintSection("Network")
	PrintKeyValue("Tracker", downloadTracker)
	if len(pieceHashes) > 0 {
		PrintStatus("Verify", "enabled", Green)
	} else {
		PrintStatus("Verify", "disabled", Yellow)
	}

	PrintDivider()
	PrintInfo("Connecting to peers...")

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
