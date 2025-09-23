package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pixperk/pixtorrent/tracker"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("=== PiXTorrent Tracker Server ===")

	redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvOrDefault("REDIS_PASSWORD", "")
	redisDB := 0
	trackerAddr := getEnvOrDefault("TRACKER_ADDR", ":8080")

	log.Printf("Connecting to Redis at %s (DB: %d)", redisAddr, redisDB)
	log.Printf("Starting tracker server on %s", trackerAddr)

	ctx := context.Background()
	storage := tracker.NewRedisStorage(ctx, redisAddr, redisPassword, redisDB)

	testClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	_, err := testClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	testClient.Close()
	log.Println("connected to Redis successfully")

	trackerServer := tracker.NewTracker(trackerAddr, storage)

	go func() {
		log.Printf("Tracker server starting on %s", trackerAddr)
		log.Println("Endpoints:")
		log.Println("  - GET /announce - BitTorrent announce endpoint")
		log.Println("  - GET /scrape   - BitTorrent scrape endpoint")

		if err := trackerServer.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	setupGracefulShutdown(trackerServer, storage)
}

func setupGracefulShutdown(trackerServer *tracker.Tracker, storage tracker.Storage) {

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := trackerServer.Server.Shutdown(ctx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	} else {
		log.Println("HTTP server shut down")
	}

	if err := storage.Close(); err != nil {
		log.Printf("Error closing storage: %v", err)
	} else {
		log.Println("Storage connections closed")
	}

	log.Println("Tracker server stopped")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
