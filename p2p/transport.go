package p2p

import "net"

//Peer represents the remote node in this case
type Peer interface {
	net.Conn
	Send([]byte) error
	CloseStream()
}

//Transport is an abstract implementation and can be adopted by any protocol such as TCP, UDP, etc
type Transport interface {
	Addr() string
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
