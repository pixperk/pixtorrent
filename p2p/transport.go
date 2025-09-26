package p2p

import "net"

type Peer interface {
	net.Conn
	SetID([20]byte)
	Send([]byte) error
	ID() [20]byte
}

type Transport interface {
	Addr() string
	Port() int
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
	Dial(addr string) error
}
