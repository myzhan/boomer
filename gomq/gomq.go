package gomq

import (
	"net"
	"strings"
	"time"

	"github.com/myzhan/boomer/gomq/zmtp"
)

var (
	defaultRetry = 250 * time.Millisecond
)

// Connection is a gomq connection. It holds
// both the net.Conn transport as well as the
// zmtp connection information.
type Connection struct {
	net  net.Conn
	zmtp *zmtp.Connection
}

// NewConnection accepts a net.Conn, a *zmtp.Connection
// and returns a *gomq.Connection.
func NewConnection(netConn net.Conn, zmtpConn *zmtp.Connection) *Connection {
	conn := &Connection{
		net:  netConn,
		zmtp: zmtpConn,
	}
	return conn
}

// ZeroMQSocket is the base gomq interface.
type ZeroMQSocket interface {
	Recv() ([]byte, error)
	Send([]byte) error
	RetryInterval() time.Duration
	SocketType() zmtp.SocketType
	SecurityMechanism() zmtp.SecurityMechanism
	AddConnection(*Connection)
	RemoveConnection(string)
	RecvChannel() chan *zmtp.Message
	Close()
}

// Client is a gomq interface used for client sockets.
// It implements the Socket interface along with a
// Connect method for connecting to endpoints.
type Client interface {
	ZeroMQSocket
	Connect(endpoint string) error
}

// ConnectClient accepts a Client interface and an endpoint
// in the format <proto>://<address>:<port>. It then attempts
// to connect to the endpoint and perform a ZMTP handshake.
func ConnectClient(c Client, endpoint string) error {
	parts := strings.Split(endpoint, "://")

Connect:
	netConn, err := net.Dial(parts[0], parts[1])
	if err != nil {
		time.Sleep(c.RetryInterval())
		goto Connect
	}

	zmtpConn := zmtp.NewConnection(netConn)
	_, err = zmtpConn.Prepare(c.SecurityMechanism(), c.SocketType(), false, nil)
	if err != nil {
		return err
	}

	conn := &Connection{
		net:  netConn,
		zmtp: zmtpConn,
	}

	c.AddConnection(conn)
	zmtpConn.Recv(c.RecvChannel())
	return nil
}

// Server is a gomq interface used for server sockets.
// It implements the Socket interface along with a
// Bind method for binding to endpoints.
type Server interface {
	ZeroMQSocket
	Bind(endpoint string) (net.Addr, error)
}

// BindServer accepts a Server interface and an endpoint
// in the format <proto>://<address>:<port>. It then attempts
// to bind to the endpoint. TODO: change this to starting
// a listener on the endpoint that performs handshakes
// with any client that connects
func BindServer(s Server, endpoint string) (net.Addr, error) {
	var addr net.Addr
	parts := strings.Split(endpoint, "://")

	ln, err := net.Listen(parts[0], parts[1])
	if err != nil {
		return addr, err
	}

	netConn, err := ln.Accept()
	if err != nil {
		return addr, err
	}

	zmtpConn := zmtp.NewConnection(netConn)
	_, err = zmtpConn.Prepare(s.SecurityMechanism(), s.SocketType(), true, nil)
	if err != nil {
		return netConn.LocalAddr(), err
	}

	conn := &Connection{
		net:  netConn,
		zmtp: zmtpConn,
	}

	s.AddConnection(conn)
	zmtpConn.Recv(s.RecvChannel())
	return netConn.LocalAddr(), nil
}
