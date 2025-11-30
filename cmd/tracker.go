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

	if trackerMemory {
		store = tracker.NewMemoryStorage()
		fmt.Println("  Storage:    in-memory")
	} else {
		store = tracker.NewRedisStorage(
			context.Background(),
			trackerRedis,
			trackerRedisPwd,
			trackerRedisDB,
		)
		fmt.Printf("  Storage:    redis://%s/%d\n", trackerRedis, trackerRedisDB)
	}

	t := tracker.NewTracker(trackerAddr, store)

	fmt.Println()
	fmt.Println("  pixTorrent Tracker")
	fmt.Println("  ------------------")
	fmt.Printf("  Address:    %s\n", trackerAddr)
	fmt.Println()
	fmt.Println("  Endpoints:")
	fmt.Printf("    GET http://%s/announce\n", trackerAddr)
	fmt.Printf("    GET http://%s/scrape\n", trackerAddr)
	fmt.Println()

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
