package p2p

import "net"

type Peer interface {
	net.Conn
	SetID(string)
	Send([]byte) error
	ID() string
}

type Transport interface {
	Addr() string
	Port() int
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
	Dial(addr string) error
}
