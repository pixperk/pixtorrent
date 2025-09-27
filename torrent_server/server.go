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
}

type TorrentServer struct {
	TorrentServerOpts
	swarm *p2p.Swarm

	peerID [20]byte

	quitch chan struct{}

	bootstrapNodes []string
}

func NewTorrentServer(opts TorrentServerOpts) *TorrentServer {
	ts := &TorrentServer{
		TorrentServerOpts: opts,
		peerID:            newPeerId(),
		quitch:            make(chan struct{}),
	}

	ts.swarm = p2p.NewSwarm(opts.TCPTransportOpts.InfoHash)

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

// Request a piece (simulate sending a request RPC)
func (ts *TorrentServer) RequestPiece(pieceIndex int) error {
	fmt.Println("RequestPiece called for piece index:", pieceIndex)
	payload := append([]byte{p2p.MsgRequestPiece}, byte(pieceIndex))
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to request piece from %s: %v\n", peer.ID(), err)
		}
	}
	return nil
}

// Respond to a piece request
func (ts *TorrentServer) SendPiece(pieceIndex int, peerID [20]byte) error {
	payload := append([]byte{p2p.MsgSendPiece}, []byte(fmt.Sprintf("piece-%d-data", pieceIndex))...)
	peer, exists := ts.swarm.GetPeer(peerID)
	if !exists {
		return fmt.Errorf("peer %x not found", peerID)
	}
	if err := peer.Send(payload); err != nil {
		return fmt.Errorf("failed to send piece to %x: %v", peerID, err)
	}
	return nil
}

// Announce that we have a piece (Have message)
func (ts *TorrentServer) AnnounceHave(pieceIndex int) error {
	payload := append([]byte{p2p.MsgHave}, byte(pieceIndex))
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to announce have to %s: %v\n", peer.ID(), err)
		}
	}
	return nil
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

			//TODO : implement handling

			msgType := rpc.Payload[0]
			payloadData := rpc.Payload[1:]
			fromaddr, fromid := rpc.From.Addr, rpc.From.PeerID
			switch msgType {
			case p2p.MsgInterested:
				fmt.Printf("[INTERESTED] from [Peer -> ID %x ; Addr %s]\n", fromid, fromaddr)
			case p2p.MsgRequestPiece:
				fmt.Printf("[REQUEST PIECE] from [Peer -> ID %x ; Addr %s], data: %x\n", fromid, fromaddr, payloadData)
			case p2p.MsgSendPiece:
				fmt.Printf("[SEND PIECE] from [Peer -> ID %x ; Addr %s], data len: %d\n", fromid, fromaddr, len(payloadData))
			case p2p.MsgHave:
				fmt.Printf("[HAVE] from [Peer -> ID %x ; Addr %s], piece index: %x\n", fromid, fromaddr, payloadData)
			case p2p.MsgBitfield:
				fmt.Printf("[BITFIELD] from [Peer -> ID %x ; Addr %s], data: %x\n", fromid, fromaddr, payloadData)
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
