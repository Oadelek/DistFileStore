package p2p

// HandshakeFunc is a type definition for a function that takes a Peer and returns an error
type HandshakeFunc func(Peer) error

// NOPHandshakeFunc is a no-operation handshake function that always returns nil
func NOPHandshakeFunc(Peer) error { return nil }
