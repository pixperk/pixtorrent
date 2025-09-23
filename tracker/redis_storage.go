package tracker

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStorage(ctx context.Context, addr, password string, db int) *RedisStorage {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisStorage{
		client: rdb,
		ctx:    ctx,
	}
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}

func (r *RedisStorage) AddPeer(infoHash [20]byte, peer *Peer) error {
	infoHashStr := hex.EncodeToString(infoHash[:])

	peerKey := fmt.Sprintf("peer:%s", peer.ID)
	torrentPeersKey := fmt.Sprintf("torrent:%s:peers", infoHashStr)

	pipe := r.client.Pipeline()

	pipe.HSet(r.ctx, peerKey, map[string]interface{}{
		"ip":         peer.IP,
		"port":       peer.Port,
		"uploaded":   peer.Uploaded,
		"downloaded": peer.Downloaded,
		"left":       peer.Left,
		"last_seen":  peer.LastSeen.Unix(),
	})

	pipe.SAdd(r.ctx, torrentPeersKey, peer.ID)

	pipe.Expire(r.ctx, peerKey, 30*time.Minute)
	pipe.Expire(r.ctx, torrentPeersKey, 2*time.Hour)

	_, err := pipe.Exec(r.ctx)
	return err
}

func (r *RedisStorage) RemovePeer(infoHash [20]byte, peerID string) error {
	infoHashStr := hex.EncodeToString(infoHash[:])

	peerKey := fmt.Sprintf("peer:%s", peerID)
	torrentPeersKey := fmt.Sprintf("torrent:%s:peers", infoHashStr)

	pipe := r.client.Pipeline()
	pipe.Del(r.ctx, peerKey)
	pipe.SRem(r.ctx, torrentPeersKey, peerID)

	_, err := pipe.Exec(r.ctx)
	return err
}

func (r *RedisStorage) GetPeer(peerID string) (*Peer, error) {
	peerKey := fmt.Sprintf("peer:%s", peerID)

	result, err := r.client.HGetAll(r.ctx, peerKey).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}

	port, _ := strconv.Atoi(result["port"])
	uploaded, _ := strconv.ParseInt(result["uploaded"], 10, 64)
	downloaded, _ := strconv.ParseInt(result["downloaded"], 10, 64)
	left, _ := strconv.ParseInt(result["left"], 10, 64)
	lastSeenUnix, _ := strconv.ParseInt(result["last_seen"], 10, 64)

	return &Peer{
		ID:         peerID,
		IP:         result["ip"],
		Port:       port,
		Uploaded:   uploaded,
		Downloaded: downloaded,
		Left:       left,
		LastSeen:   time.Unix(lastSeenUnix, 0),
	}, nil
}

func (r *RedisStorage) GetPeers(infoHash [20]byte, maxPeers int) ([]*Peer, error) {
	infoHashStr := hex.EncodeToString(infoHash[:])
	torrentPeersKey := fmt.Sprintf("torrent:%s:peers", infoHashStr)

	peerIDs, err := r.client.SRandMemberN(r.ctx, torrentPeersKey, int64(maxPeers)).Result()
	if err != nil {
		return nil, err
	}

	peers := make([]*Peer, 0, len(peerIDs))

	for _, peerID := range peerIDs {
		peer, err := r.GetPeer(peerID)
		if err != nil {
			continue
		}
		peers = append(peers, peer)
	}

	return peers, nil
}

func (r *RedisStorage) GetSeeders(infoHash [20]byte, maxPeers int) ([]*Peer, error) {
	peers, err := r.GetPeers(infoHash, maxPeers*2)
	if err != nil {
		return nil, err
	}

	seeders := make([]*Peer, 0)
	count := 0

	for _, peer := range peers {
		if peer.Left == 0 && count < maxPeers {
			seeders = append(seeders, peer)
			count++
		}
	}

	return seeders, nil
}

func (r *RedisStorage) GetLeechers(infoHash [20]byte, maxPeers int) ([]*Peer, error) {
	peers, err := r.GetPeers(infoHash, maxPeers*2)
	if err != nil {
		return nil, err
	}

	leechers := make([]*Peer, 0)
	count := 0

	for _, peer := range peers {
		if peer.Left > 0 && count < maxPeers {
			leechers = append(leechers, peer)
			count++
		}
	}

	return leechers, nil
}

func (r *RedisStorage) GetTorrentStats(infoHash [20]byte) (*TorrentInfo, error) {
	infoHashStr := hex.EncodeToString(infoHash[:])
	torrentPeersKey := fmt.Sprintf("torrent:%s:peers", infoHashStr)
	torrentStatsKey := fmt.Sprintf("torrent:%s:stats", infoHashStr)

	peerIDs, err := r.client.SMembers(r.ctx, torrentPeersKey).Result()
	if err != nil {
		return nil, err
	}

	seeders := make([]string, 0)
	leechers := make([]string, 0)

	for _, peerID := range peerIDs {
		peer, err := r.GetPeer(peerID)
		if err != nil {
			continue
		}

		if peer.Left == 0 {
			seeders = append(seeders, peerID)
		} else {
			leechers = append(leechers, peerID)
		}
	}

	completedStr, err := r.client.HGet(r.ctx, torrentStatsKey, "completed").Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	completed := 0
	if completedStr != "" {
		completed, _ = strconv.Atoi(completedStr)
	}

	return &TorrentInfo{
		InfoHash:  infoHash,
		Seeders:   seeders,
		Leechers:  leechers,
		Completed: completed,
	}, nil
}

func (r *RedisStorage) IncrementCompleted(infoHash [20]byte) error {
	infoHashStr := hex.EncodeToString(infoHash[:])
	torrentStatsKey := fmt.Sprintf("torrent:%s:stats", infoHashStr)

	return r.client.HIncrBy(r.ctx, torrentStatsKey, "completed", 1).Err()
}

func (r *RedisStorage) UpdatePeerLastSeen(peerID string) error {
	peerKey := fmt.Sprintf("peer:%s", peerID)

	return r.client.HSet(r.ctx, peerKey, "last_seen", time.Now().Unix()).Err()
}

func (r *RedisStorage) CleanupExpiredPeers() error {
	keys, err := r.client.Keys(r.ctx, "peer:*").Result()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-30 * time.Minute).Unix()

	for _, key := range keys {
		lastSeenStr, err := r.client.HGet(r.ctx, key, "last_seen").Result()
		if err != nil {
			continue
		}

		lastSeen, err := strconv.ParseInt(lastSeenStr, 10, 64)
		if err != nil {
			continue
		}

		if lastSeen < cutoff {
			peerID := key[5:]

			torrentKeys, _ := r.client.Keys(r.ctx, "torrent:*:peers").Result()
			for _, torrentKey := range torrentKeys {
				r.client.SRem(r.ctx, torrentKey, peerID)
			}

			r.client.Del(r.ctx, key)
		}
	}

	return nil
}

func (r *RedisStorage) GetActiveTorrents() ([]string, error) {
	keys, err := r.client.Keys(r.ctx, "torrent:*:peers").Result()
	if err != nil {
		return nil, err
	}

	torrents := make([]string, 0, len(keys))
	for _, key := range keys {
		if len(key) > 17 {
			infoHash := key[8 : len(key)-6]
			torrents = append(torrents, infoHash)
		}
	}

	return torrents, nil
}
