package p2p

const (
	IncomingMessage = 0x1
	IncomingStream  = 0x2
)

// This RPC struct is to hold arbitrary data sent over the transport layer
type RPC struct {
	From    string
	Payload []byte
	Stream  bool
}
