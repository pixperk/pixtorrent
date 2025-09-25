package p2p

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
)

type TCPPeer struct {
	net.Conn
	id       string
	outbound bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{Conn: conn, outbound: outbound}
}

func (p *TCPPeer) Send(data []byte) error {
	_, err := p.Write(data)
	return err
}

func (p *TCPPeer) SetID(id string) {
	p.id = id
}

func (p *TCPPeer) ID() string {
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
	localID  string // our own peer ID
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
		fmt.Printf("dropping peer connection : %s", err)
		conn.Close()
	}()
	peer := NewTCPPeer(conn, outbound)
	if t.Handshake != nil {
		if err = t.Handshake(peer, t.InfoHash, t.localID); err != nil {
			return
		}
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	//read loop
	for {
		rpc := RPC{}
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			return
		}

		rpc.From = conn.RemoteAddr().String()

		t.rpcch <- rpc
	}
}

func generate20ByteID() string {
	id := make([]byte, 20)
	if _, err := rand.Read(id); err != nil {
		panic(err) // should never happen
	}
	return string(id) // raw bytes
}
