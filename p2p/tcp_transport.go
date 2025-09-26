package p2p

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

type TCPPeer struct {
	net.Conn
	id [20]byte

	mu       sync.Mutex
	outbound bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{Conn: conn, outbound: outbound}
}

func (p *TCPPeer) Send(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("sender triggered")

	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)))
	copy(buf[4:], data)

	n, err := p.Write(buf)
	log.Printf("[SEND] %d bytes to %s, payload: %x", n, p.RemoteAddr(), buf)

	return err
}

func (p *TCPPeer) SetID(id [20]byte) {
	p.id = id
}

func (p *TCPPeer) ID() [20]byte {
	return p.id
}

type OnPeerFunc func(Peer) error

type TCPTransportOpts struct {
	ListenAddr string
	Handshake  HandshakeFunc
	Decoder    Decoder
	OnPeer     OnPeerFunc
	InfoHash   [20]byte
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcch    chan RPC
	localID  [20]byte // our own peer ID
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcch:            make(chan RPC, 1024),
		localID:          generate20ByteID(),
	}
}

func (t *TCPTransport) Addr() string {
	return t.listener.Addr().String()
}

func (t *TCPTransport) Port() int {
	return t.listener.Addr().(*net.TCPAddr).Port
}

func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	ln, err := net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}
	t.listener = ln
	go t.acceptLoop()

	log.Printf("piXTorrent tcp peer listening on %s\n", t.listener.Addr().String())

	return nil
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}

			fmt.Printf("tcp accept error: %v\n", err)

		}

		fmt.Printf("new tcp connection from %s\n", conn.RemoteAddr().String())

		go t.handleConn(conn, false)
	}

}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error
	defer func() {
		if err != nil {
			fmt.Println("dropping peer connection:", err)
		}
		conn.Close()
	}()

	peer := NewTCPPeer(conn, outbound)

	// Perform handshake
	if t.Handshake != nil {
		if err = t.Handshake(peer, t.InfoHash, t.localID, outbound); err != nil {
			fmt.Println("handshake failed:", err)
			return
		}
	}

	// Prevent self-connection
	peerId := peer.ID()
	if bytes.Equal(peerId[:], t.localID[:]) {
		fmt.Println("dropping peer connection: connected to self, peer ID:", peerId)
		return
	}

	// Register peer in swarm
	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			// OnPeer can reject duplicates or invalid peers
			fmt.Println("dropping peer connection:", err)
			return
		}
	}

	// Read loop
	for {
		log.Println("we here")
		rpc := RPC{}
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			fmt.Printf("[%s] decode error: %v\n", conn.RemoteAddr(), err)
			return
		}

		rpc.From = conn.RemoteAddr().String()
		fmt.Printf("[%s] received RPC: %+v\n", rpc.From, rpc)

		select {
		case t.rpcch <- rpc:
		default:
			fmt.Println("rpc channel full, dropping message")
			return
		}
	}
}

func generate20ByteID() [20]byte {
	var id [20]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return id
}
