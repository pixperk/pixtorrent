package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/pixperk/pixtorrent/meta"
)

type TrackerClient struct {
	client     *http.Client
	peerID     string
	port       int
	uploaded   int64
	downloaded int64
}

type AnnounceResponse struct {
	Interval int
	Peers    []Peer
}

type Peer struct {
	PeerID string
	IP     string
	Port   int
}

func NewTrackerClient(peerID string, port int) *TrackerClient {
	return &TrackerClient{
		client: &http.Client{Timeout: 30 * time.Second},
		peerID: peerID,
		port:   port,
	}
}

func (tc *TrackerClient) Announce(trackerURL string, infoHash [20]byte, left int64, event string) (*AnnounceResponse, error) {

	announceURL, err := tc.buildAnnounceURL(trackerURL, infoHash, left, event)
	if err != nil {
		return nil, fmt.Errorf("failed to build announce URL: %w", err)
	}

	resp, err := tc.client.Get(announceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to make announce request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	decoder := meta.NewDecoder(bytes.NewReader(body))
	decoded, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %w", err)
	}

	return tc.parseAnnounceResponse(decoded)
}

func (tc *TrackerClient) buildAnnounceURL(trackerURL string, infoHash [20]byte, left int64, event string) (string, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return "", err
	}

	if u.Path == "" {
		u.Path = "/announce"
	}

	params := url.Values{}
	params.Set("info_hash", string(infoHash[:]))
	params.Set("peer_id", tc.peerID)
	params.Set("port", strconv.Itoa(tc.port))
	params.Set("uploaded", strconv.FormatInt(tc.uploaded, 10))
	params.Set("downloaded", strconv.FormatInt(tc.downloaded, 10))
	params.Set("left", strconv.FormatInt(left, 10))
	params.Set("compact", "0")
	params.Set("numwant", "50")

	if event != "" {
		params.Set("event", event)
	}

	u.RawQuery = params.Encode()
	return u.String(), nil
}

func (tc *TrackerClient) parseAnnounceResponse(decoded meta.Node) (*AnnounceResponse, error) {
	dict, ok := decoded.(meta.BDict)
	if !ok {
		return nil, fmt.Errorf("expected dictionary response")
	}

	if failureReason, exists := dict["failure reason"]; exists {
		if reasonStr, ok := failureReason.(meta.BString); ok {
			return nil, fmt.Errorf("tracker error: %s", string(reasonStr))
		}
	}

	response := &AnnounceResponse{}

	// Parse interval
	if intervalNode, exists := dict["interval"]; exists {
		if interval, ok := intervalNode.(meta.BInt); ok {
			response.Interval = int(interval)
		}
	}

	// Parse peers list
	if peersNode, exists := dict["peers"]; exists {
		if peersList, ok := peersNode.(meta.BList); ok {
			for _, peerNode := range peersList {
				if peerDict, ok := peerNode.(meta.BDict); ok {
					peer := Peer{}

					if peerIDNode, exists := peerDict["peer id"]; exists {
						if peerID, ok := peerIDNode.(meta.BString); ok {
							peer.PeerID = string(peerID)
						}
					}

					if ipNode, exists := peerDict["ip"]; exists {
						if ip, ok := ipNode.(meta.BString); ok {
							peer.IP = string(ip)
						}
					}

					if portNode, exists := peerDict["port"]; exists {
						if port, ok := portNode.(meta.BInt); ok {
							peer.Port = int(port)
						}
					}

					response.Peers = append(response.Peers, peer)
				}
			}
		}
	}

	return response, nil
}

type ScrapeResponse struct {
	Complete   int
	Incomplete int
	Downloaded int
}

func (tc *TrackerClient) Scrape(trackerURL string, infoHash [20]byte) (*ScrapeResponse, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/scrape"
	params := url.Values{}
	params.Set("info_hash", string(infoHash[:]))
	u.RawQuery = params.Encode()

	resp, err := tc.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decoder := meta.NewDecoder(bytes.NewReader(body))
	decoded, err := decoder.Decode()
	if err != nil {
		return nil, err
	}

	dict, ok := decoded.(meta.BDict)
	if !ok {
		return nil, fmt.Errorf("invalid scrape response")
	}
	files, ok := dict["files"].(meta.BDict)
	if !ok {
		return nil, fmt.Errorf("missing files in scrape response")
	}
	for _, v := range files {
		stats, ok := v.(meta.BDict)
		if !ok {
			continue
		}
		response := &ScrapeResponse{}
		if complete, ok := stats["complete"].(meta.BInt); ok {
			response.Complete = int(complete)
		}
		if incomplete, ok := stats["incomplete"].(meta.BInt); ok {
			response.Incomplete = int(incomplete)
		}
		if downloaded, ok := stats["downloaded"].(meta.BInt); ok {
			response.Downloaded = int(downloaded)
		}
		return response, nil
	}
	return nil, fmt.Errorf("no stats found")
}

func (tc *TrackerClient) UpdateStats(uploaded, downloaded int64) {
	tc.uploaded = uploaded
	tc.downloaded = downloaded
}
