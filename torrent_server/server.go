package torrentserver

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/pixperk/pixtorrent/client"
	"github.com/pixperk/pixtorrent/p2p"
)

type TorrentServerOpts struct {
	Transport        p2p.Transport
	TCPTransportOpts p2p.TCPTransportOpts
	TrackerUrl       string
	RootDir          string
	FileFormat       string
}

type TorrentServer struct {
	TorrentServerOpts
	swarm *p2p.Swarm

	peerID [20]byte

	quitch chan struct{}

	bootstrapNodes  []string
	trackerClient   *client.TrackerClient
	pendingRequests map[[20]byte][]int
	pendingMu       sync.Mutex
}

func NewTorrentServer(opts TorrentServerOpts, pieceMgr *p2p.PieceManager) *TorrentServer {
	ts := &TorrentServer{
		TorrentServerOpts: opts,
		peerID:            newPeerId(),
		quitch:            make(chan struct{}),
		pendingRequests:   make(map[[20]byte][]int),
	}

	ts.swarm = p2p.NewSwarm(ts.peerID, opts.TCPTransportOpts.InfoHash, pieceMgr)

	// Initialize tracker client
	hexEncodedID := fmt.Sprintf("%x", ts.peerID)
	ts.trackerClient = client.NewTrackerClient(hexEncodedID, 0) // port will be set after transport starts

	if opts.Transport != nil {
		ts.Transport = opts.Transport
		if tt, ok := ts.Transport.(*p2p.TCPTransport); ok {
			tt.OnPeer = ts.swarm.OnPeer
		}
	} else {
		tcpTransport := p2p.NewTCPTransport(opts.TCPTransportOpts)
		tcpTransport.OnPeer = ts.swarm.OnPeer
		ts.Transport = tcpTransport
	}

	return ts
}

func (ts *TorrentServer) Swarm() *p2p.Swarm {
	return ts.swarm
}

func (ts *TorrentServer) PeerID() [20]byte {
	return ts.peerID
}

func (ts *TorrentServer) Start() error {
	if err := ts.Transport.ListenAndAccept(); err != nil {
		return err
	}

	// Update tracker client with actual port after transport starts
	hexEncodedID := fmt.Sprintf("%x", ts.peerID)
	ts.trackerClient = client.NewTrackerClient(hexEncodedID, ts.Transport.Port())

	time.Sleep(500 * time.Millisecond)

	if err := ts.bootstrapNetwork(); err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)

	ts.loop()
	return nil
}

func (ts *TorrentServer) Stop() {
	close(ts.quitch)
	ts.Transport.Close()
}

func (ts *TorrentServer) loop() {
	defer func() {
		log.Println("torrent server stopped")
		ts.Stop()
	}()

	announceTicker := time.NewTicker(5 * time.Minute)
	defer announceTicker.Stop()

	unchokeTicker := time.NewTicker(10 * time.Second)
	defer unchokeTicker.Stop()

	for {
		select {
		case rpc := <-ts.Transport.Consume():
			if len(rpc.Payload) == 0 {
				fmt.Printf("[KEEP-ALIVE] from %s\n", rpc.From)
				continue
			}

			msgType := rpc.Payload[0]
			payloadData := rpc.Payload[1:]
			fromaddr, fromid := rpc.From.Addr, rpc.From.PeerID
			switch msgType {
			case p2p.MsgInterested:
				fmt.Printf("[INTERESTED] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
				ts.swarm.SetPeerInterested(fromid, true)
			case p2p.MsgNotInterested:
				fmt.Printf("[NOT INTERESTED] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
				ts.swarm.SetPeerInterested(fromid, false)
			case p2p.MsgRequestPiece:
				if len(payloadData) < 4 {
					fmt.Printf("[ERROR] MsgRequestPiece payload too short: %d bytes\n", len(payloadData))
					continue
				}
				pieceIdx := int(binary.BigEndian.Uint32(payloadData[:4]))
				ts.handlePieceRequest(rpc, pieceIdx)
			case p2p.MsgSendPiece:
				ts.handlePiece(rpc, payloadData)

			case p2p.MsgHave:
				if len(payloadData) < 4 {
					fmt.Printf("[ERROR] MsgHave payload too short: %d bytes\n", len(payloadData))
					continue
				}
				pieceIdx := int(binary.BigEndian.Uint32(payloadData[:4]))
				fmt.Printf("[HAVE] from [Peer -> ID %x ; Addr %s], piece index: %d\n", fromid, fromaddr, pieceIdx)
				ts.swarm.SetPeerHasPiece(fromid, pieceIdx)
			case p2p.MsgBitfield:
				ts.handleBitfieldAnnouncement(rpc, payloadData)
			case p2p.MsgChoke:
				fmt.Printf("[CHOKE] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
				ts.swarm.SetPeerChoking(fromid, true)
			case p2p.MsgUnchoke:
				fmt.Printf("[UNCHOKE] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
				ts.swarm.SetPeerChoking(fromid, false)
				// Now we can send pending piece requests
				ts.sendPendingRequests(fromid)
			default:
				fmt.Printf("[UNKNOWN MSG %d] from [Peer -> ID %x ; Addr %s], data: %x\n", msgType, fromid, fromaddr, payloadData)
			}

		case <-unchokeTicker.C:
			ts.runUnchokeRound()

		case <-announceTicker.C:
			go func() {
				if err := ts.AnnounceToTracker(""); err != nil {
					fmt.Printf("Periodic tracker announce failed: %v\n", err)
				}
			}()

		case <-ts.quitch:
			if err := ts.AnnounceToTracker("stopped"); err != nil {
				fmt.Printf("Failed to announce stop to tracker: %v\n", err)
			}
			return
		}
	}
}

func (ts *TorrentServer) runUnchokeRound() {
	actions := ts.swarm.RunUnchokeAlgorithm()

	for _, action := range actions {
		peer, exists := ts.swarm.GetPeer(action.PeerID)
		if !exists {
			continue
		}

		var msg []byte
		if action.Unchoke {
			msg = []byte{p2p.MsgUnchoke}
			fmt.Printf("[UNCHOKING] peer %x\n", action.PeerID)
		} else {
			msg = []byte{p2p.MsgChoke}
			fmt.Printf("[CHOKING] peer %x\n", action.PeerID)
		}

		if err := peer.Send(msg); err != nil {
			fmt.Printf("failed to send choke/unchoke to %x: %v\n", action.PeerID, err)
		}
	}
}

func (ts *TorrentServer) bootstrapNetwork() error {
	if err := ts.populateBootstrapNodes(); err != nil {
		return err
	}

	for _, addr := range ts.bootstrapNodes {

		go func(addr string) {
			if err := ts.Transport.Dial(addr); err != nil {
				log.Printf("Failed to dial bootstrap node %s: %v", addr, err)
			}
		}(addr)
	}

	return nil
}

func (ts *TorrentServer) populateBootstrapNodes() error {
	resp, err := ts.trackerClient.Announce(ts.TrackerUrl, ts.TCPTransportOpts.InfoHash, 0, "started")
	if err != nil {
		return err
	}

	unique := make(map[string]struct{})
	for _, p := range resp.Peers {
		addr := formatAddr(p.IP, p.Port)
		if addr == ts.Transport.Addr() {
			continue
		}

		if ts.Transport.Addr() > addr {
			continue
		}

		if _, exists := unique[addr]; !exists {
			unique[addr] = struct{}{}
			ts.bootstrapNodes = append(ts.bootstrapNodes, addr)
		}
	}

	return nil
}

func formatAddr(ip string, port int) string {
	if strings.Contains(ip, ":") { // ipv6
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func newPeerId() [20]byte {
	id := [20]byte{}

	// dynamic prefix: -XXYYYY-
	id[0] = '-'
	id[1] = randomLetter()
	id[2] = randomLetter()
	for i := 3; i <= 6; i++ {
		id[i] = randomDigit()
	}
	id[7] = '-'

	// remaining 12 bytes random
	_, err := rand.Read(id[8:])
	if err != nil {
		panic(err)
	}

	return id
}

func randomLetter() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(26))
	return 'A' + byte(n.Int64())
}

func randomDigit() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(10))
	return '0' + byte(n.Int64())
}
