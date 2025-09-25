package torrentserver

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strconv"

	"github.com/pixperk/pixtorrent/client"
	"github.com/pixperk/pixtorrent/p2p"
)

type TorrentServerOpts struct {
	Transport        p2p.Transport
	TCPTransportOpts p2p.TCPTransportOpts
}

type TorrentServer struct {
	TorrentServerOpts
	swarm *p2p.Swarm

	peerID string

	trackerUrl string

	quitch chan struct{}
}

func NewTorrentServer(opts TorrentServerOpts) *TorrentServer {
	ts := &TorrentServer{
		TorrentServerOpts: opts,
		swarm:             p2p.NewSwarm(opts.TCPTransportOpts.InfoHash),
		peerID:            newPeerId(),
		quitch:            make(chan struct{}),
	}

	opts.TCPTransportOpts.OnPeer = ts.swarm.OnPeer

	return ts
}

// Request a piece (simulate sending a request RPC)
func (ts *TorrentServer) RequestPiece(pieceIndex int) error {
	payload := append([]byte{p2p.MsgRequestPiece}, byte(pieceIndex))
	for _, peer := range ts.swarm.Peers() {
		if err := peer.Send(payload); err != nil {
			fmt.Printf("failed to request piece from %s: %v\n", peer.ID(), err)
		}
	}
	return nil
}

// Respond to a piece request
func (ts *TorrentServer) SendPiece(pieceIndex int, peerID string) error {
	payload := append([]byte{p2p.MsgSendPiece}, []byte(fmt.Sprintf("piece-%d-data", pieceIndex))...)
	peer, exists := ts.swarm.GetPeer(peerID)
	if !exists {
		return fmt.Errorf("peer %s not found", peerID)
	}
	if err := peer.Send(payload); err != nil {
		return fmt.Errorf("failed to send piece to %s: %v", peerID, err)
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

	if err := ts.bootstrapNetwork(); err != nil {
		return err
	}

	ts.loop()
	return nil
}

func (ts *TorrentServer) loop() {
	defer func() {
		log.Println("torrent server stopped")
		ts.Transport.Close()
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

			switch msgType {
			case p2p.MsgInterested:
				fmt.Printf("[INTERESTED] from %s\n", rpc.From)
			case p2p.MsgRequestPiece:
				fmt.Printf("[REQUEST PIECE] from %s, data: %x\n", rpc.From, payloadData)
			case p2p.MsgSendPiece:
				fmt.Printf("[SEND PIECE] from %s, data len: %d\n", rpc.From, len(payloadData))
			case p2p.MsgHave:
				fmt.Printf("[HAVE] from %s, piece index: %x\n", rpc.From, payloadData)
			case p2p.MsgBitfield:
				fmt.Printf("[BITFIELD] from %s, data: %x\n", rpc.From, payloadData)
			case p2p.MsgChoke:
				fmt.Printf("[CHOKE] from %s\n", rpc.From)
			case p2p.MsgUnchoke:
				fmt.Printf("[UNCHOKE] from %s\n", rpc.From)
			default:
				fmt.Printf("[UNKNOWN MSG %d] from %s, data: %x\n", msgType, rpc.From, payloadData)
			}

		case <-ts.quitch:
			return
		}
	}
}

func (ts *TorrentServer) bootstrapNetwork() error {
	//announce to tracker and get peers
	tc := client.NewTrackerClient(
		ts.peerID,
		ts.Transport.Port(),
	)

	resp, err := tc.Announce(ts.trackerUrl, ts.TCPTransportOpts.InfoHash, 0, "started")
	if err != nil {
		return err
	}

	for _, p := range resp.Peers {
		addr := p.IP + ":" + strconv.Itoa(p.Port)
		if err := ts.Transport.Dial(addr); err != nil {
			log.Printf("[%s] dial error %s: %v", p.PeerID, addr, err)
		}
	}

	return nil
}

func newPeerId() string {
	id := make([]byte, 20)

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

	return string(id)
}

func randomLetter() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(26))
	return 'A' + byte(n.Int64())
}

func randomDigit() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(10))
	return '0' + byte(n.Int64())
}
