package p2p

import (
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

type TCPTransportOpts struct {
	Listener      net.Listener
	HandshakeFunc HandshakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcch    chan RPC
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		listener:         opts.Listener,
		rpcch:            make(chan RPC, 1024),
	}
}

func (t *TCPTransport) Addr() net.Addr {
	return t.listener.Addr()
}

func (t *TCPTransport) Close() error {
	return t.listener.Close()
}
