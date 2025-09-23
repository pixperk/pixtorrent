package tracker

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackpal/bencode-go"
)

type Tracker struct {
	Server *http.Server
	Store  Storage
}

func NewTracker(addr string, store Storage) *Tracker {
	tracker := &Tracker{Store: store}
	mux := http.NewServeMux()

	mux.HandleFunc("/announce", tracker.handleAnnounce)
	mux.HandleFunc("/scrape", tracker.handleScrape)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	tracker.Server = server
	return tracker
}

func (t *Tracker) Start() error {
	return t.Server.ListenAndServe()
}

func (t *Tracker) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	//url : GET /announce?info_hash=%12%34%56%78%9a%bc%de%f0%12%34%56%78%9a%bc%de%f0%12%34%56%78&peer_id=-TR2940-k8hj0wgej6ch&port=51413&uploaded=245760&downloaded=1073741824&left=0&numwant=80&key=61038894&compact=1&supportcrypto=1&event=completed HTTP/1.1
	infoHashStr := r.URL.Query().Get("info_hash")
	peerID := r.URL.Query().Get("peer_id")
	portStr := r.URL.Query().Get("port")
	uploadedStr := r.URL.Query().Get("uploaded")
	downloadedStr := r.URL.Query().Get("downloaded")
	leftStr := r.URL.Query().Get("left")
	event := r.URL.Query().Get("event")
	numWantStr := r.URL.Query().Get("numwant")

	if infoHashStr == "" || peerID == "" || portStr == "" {
		sendErrorResponse(w, "Missing required parameters")
		return
	}

	infoHash, err := decodeInfoHash(infoHashStr)
	if err != nil {
		sendErrorResponse(w, "Invalid info_hash")
		return
	}

	port, _ := strconv.Atoi(portStr)
	uploaded, _ := strconv.ParseInt(uploadedStr, 10, 64)
	downloaded, _ := strconv.ParseInt(downloadedStr, 10, 64)
	left, _ := strconv.ParseInt(leftStr, 10, 64)
	numWant, _ := strconv.Atoi(numWantStr)

	if numWant == 0 {
		numWant = 50
	}

	clientIP := getClientIP(r)

	peer := &Peer{
		ID:         peerID,
		IP:         clientIP,
		Port:       port,
		Uploaded:   uploaded,
		Downloaded: downloaded,
		Left:       left,
		LastSeen:   time.Now(),
	}

	switch event {
	case "started":
		err = t.Store.AddPeer(infoHash, peer)

	case "stopped":
		err = t.Store.RemovePeer(infoHash, peerID)
		if err != nil {
			sendErrorResponse(w, "Failed to remove peer")
			return
		}
		// Don't return peer list for stopped events
		sendAnnounceResponse(w, []*Peer{}, 1800)
		return

	case "completed":
		peer.Left = 0
		err = t.Store.AddPeer(infoHash, peer)
		t.Store.IncrementCompleted(infoHash)

	default:
		// Regular update (empty event or periodic announce)
		err = t.Store.AddPeer(infoHash, peer)
	}

	if err != nil {
		sendErrorResponse(w, "Storage error")
		return
	}

	peers, err := t.Store.GetPeers(infoHash, numWant)
	if err != nil {
		sendErrorResponse(w, "Failed to get peers")
		return
	}

	filteredPeers := make([]*Peer, 0)
	for _, p := range peers {
		if p.ID != peerID {
			filteredPeers = append(filteredPeers, p)
		}
	}

	sendAnnounceResponse(w, filteredPeers, 1800)

}

func decodeInfoHash(infoHashStr string) ([20]byte, error) {

	decoded, err := url.QueryUnescape(infoHashStr)
	if err != nil {
		return [20]byte{}, err
	}

	if len(decoded) != 20 {
		return [20]byte{}, fmt.Errorf("info_hash must be 20 bytes")
	}

	var hash [20]byte
	copy(hash[:], decoded)
	return hash, nil
}

func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func sendAnnounceResponse(w http.ResponseWriter, peers []*Peer, interval int) {
	response := map[string]interface{}{
		"interval": interval,
		"peers":    convertPeersToList(peers),
	}

	data, err := encodeToBencode(response)
	if err != nil {
		http.Error(w, "Encoding error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func convertPeersToList(peers []*Peer) []map[string]interface{} {
	result := make([]map[string]interface{}, len(peers))
	for i, peer := range peers {
		result[i] = map[string]interface{}{
			"peer id": peer.ID,
			"ip":      peer.IP,
			"port":    peer.Port,
		}
	}
	return result
}

func sendErrorResponse(w http.ResponseWriter, reason string) {
	response := map[string]interface{}{
		"failure reason": reason,
	}

	data, _ := encodeToBencode(response)
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func encodeToBencode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t *Tracker) handleScrape(w http.ResponseWriter, r *http.Request) {
	// URL: GET /scrape?info_hash=%12%34%56%78%9a%bc%de%f0%12%34%56%78%9a%bc%de%f0%12%34%56%78
	infoHashParams := r.URL.Query()["info_hash"]

	files := make(map[string]interface{})

	if len(infoHashParams) == 0 {
		// TODO: Implement fetching stats for all torrents
	} else {

		for _, infoHashStr := range infoHashParams {
			infoHash, err := decodeInfoHash(infoHashStr)
			if err != nil {
				continue // Skip invalid hashes
			}

			torrentStats, err := t.Store.GetTorrentStats(infoHash)
			if err != nil {
				// If torrent not found, set zeros
				files[infoHashStr] = map[string]interface{}{
					"complete":   0,
					"incomplete": 0,
					"downloaded": 0,
				}
				continue
			}

			files[infoHashStr] = map[string]interface{}{
				"complete":   len(torrentStats.Seeders),
				"incomplete": len(torrentStats.Leechers),
				"downloaded": torrentStats.Completed,
			}
		}
	}

	response := map[string]interface{}{
		"files": files,
	}

	data, err := encodeToBencode(response)
	if err != nil {
		http.Error(w, "Encoding error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}
