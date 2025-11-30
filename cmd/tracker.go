package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pixperk/pixtorrent/tracker"
	"github.com/spf13/cobra"
)

var (
	trackerAddr     string
	trackerRedis    string
	trackerRedisDB  int
	trackerRedisPwd string
	trackerMemory   bool
)

var trackerCmd = &cobra.Command{
	Use:   "tracker",
	Short: "Start the BitTorrent tracker",
	Long:  `Start a BitTorrent tracker server that coordinates peers.`,
	RunE:  runTracker,
}

func init() {
	trackerCmd.Flags().StringVarP(&trackerAddr, "addr", "a", ":8080", "Address to listen on")
	trackerCmd.Flags().StringVarP(&trackerRedis, "redis", "r", "localhost:6379", "Redis address")
	trackerCmd.Flags().IntVarP(&trackerRedisDB, "redis-db", "d", 0, "Redis database number")
	trackerCmd.Flags().StringVarP(&trackerRedisPwd, "redis-password", "P", "", "Redis password")
	trackerCmd.Flags().BoolVarP(&trackerMemory, "memory", "m", false, "Use in-memory storage (no Redis)")

	rootCmd.AddCommand(trackerCmd)
}

func runTracker(cmd *cobra.Command, args []string) error {
	var store tracker.Storage

	PrintLogoSmall()
	PrintHeader("TRACKER")

	if trackerMemory {
		store = tracker.NewMemoryStorage()
		PrintSection("Storage")
		PrintStatus("Type", "in-memory", Yellow)
	} else {
		store = tracker.NewRedisStorage(
			context.Background(),
			trackerRedis,
			trackerRedisPwd,
			trackerRedisDB,
		)
		PrintSection("Storage")
		PrintStatus("Type", "redis", Green)
		PrintKeyValue("Address", fmt.Sprintf("%s/%d", trackerRedis, trackerRedisDB))
	}

	t := tracker.NewTracker(trackerAddr, store)

	PrintSection("Server")
	PrintKeyValueHighlight("Listen", trackerAddr)

	PrintSection("Endpoints")
	PrintKeyValue("Announce", fmt.Sprintf("http://localhost%s/announce", trackerAddr))
	PrintKeyValue("Scrape", fmt.Sprintf("http://localhost%s/scrape", trackerAddr))

	PrintDivider()
	PrintInfo("Tracker is running...")

	if !trackerMemory {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if err := store.CleanupExpiredPeers(); err != nil {
					fmt.Printf("Cleanup error: %v\n", err)
				}
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down tracker...")
		t.Server.Close()
		store.Close()
		os.Exit(0)
	}()

	return t.Start()
}
