package gomq

import "github.com/myzhan/boomer/gomq/zmtp"

// PullSocket is a ZMQ_PULL socket type.
// See: http://rfc.zeromq.org/spec:41
type PullSocket struct {
	*Socket
}

// NewPull accepts a zmtp.SecurityMechanism and returns
// a PullSocket as a gomq.Pull interface.
func NewPull(mechanism zmtp.SecurityMechanism) Client {
	return &PullSocket{
		Socket: NewSocket(false, zmtp.PullSocketType, mechanism),
	}
}

// Connect accepts a zeromq endpoint and connects the
// pull socket to it. Currently the only transport
// supported is TCP. The endpoint string should be
// in the format "tcp://<address>:<port>".
func (c *PullSocket) Connect(endpoint string) error {
	return ConnectClient(c, endpoint)
}
