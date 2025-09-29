package torrentserver

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strings"
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

	bootstrapNodes []string
}

func NewTorrentServer(opts TorrentServerOpts, pieceMgr *p2p.PieceManager) *TorrentServer {
	ts := &TorrentServer{
		TorrentServerOpts: opts,
		peerID:            newPeerId(),
		quitch:            make(chan struct{}),
	}

	ts.swarm = p2p.NewSwarm(ts.peerID, opts.TCPTransportOpts.InfoHash, pieceMgr)

	// prefer a transport supplied by caller; otherwise create one
	if opts.Transport != nil {
		ts.Transport = opts.Transport
		// try to wire OnPeer if this is actually a *TCPTransport
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
			case p2p.MsgRequestPiece:
				ts.handlePieceRequest(rpc, int(payloadData[0]))
			case p2p.MsgSendPiece:
				ts.handlePiece(rpc, payloadData)

			case p2p.MsgHave:
				fmt.Printf("[HAVE] from [Peer -> ID %x ; Addr %s], piece index: %x\n", fromid, fromaddr, payloadData)
			case p2p.MsgBitfield:
				ts.handleBitfieldAnnouncement(rpc, payloadData)
			case p2p.MsgChoke:
				fmt.Printf("[CHOKE] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
			case p2p.MsgUnchoke:
				fmt.Printf("[UNCHOKE] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
			default:
				fmt.Printf("[UNKNOWN MSG %d] from [Peer -> ID %x ; Addr %s], data: %x\n", msgType, fromid, fromaddr, payloadData)
			}

		case <-ts.quitch:
			return
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
	hexEncodedID := fmt.Sprintf("%x", ts.peerID)
	tc := client.NewTrackerClient(hexEncodedID, ts.Transport.Port())

	resp, err := tc.Announce(ts.TrackerUrl, ts.TCPTransportOpts.InfoHash, 0, "started")
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
