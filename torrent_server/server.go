package torrentserver

import (
	"crypto/rand"
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

	trackerUrl string

	quitch chan struct{}
}

func NewTorrentServer(opts TorrentServerOpts) *TorrentServer {
	return &TorrentServer{
		TorrentServerOpts: opts,
		swarm:             p2p.NewSwarm(opts.TCPTransportOpts.InfoHash),
		quitch:            make(chan struct{}),
	}
}

func (ts *TorrentServer) bootstrapPeers() error {
	//announce to tracker and get peers
	tc := client.NewTrackerClient(
		newPeerId(),
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

func randomLetter() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(26))
	return 'A' + byte(n.Int64())
}

func randomDigit() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(10))
	return '0' + byte(n.Int64())
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
