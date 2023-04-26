//go:build !goczmq
// +build !goczmq

package boomer

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/myzhan/gomq"
	"github.com/myzhan/gomq/zmtp"
)

// Router is a gomq interface used for router sockets.
// It implements the Socket interface along with a
// Bind method for binding to endpoints.
type Router interface {
	gomq.ZeroMQSocket
	Bind(endpoint string) (net.Addr, error)
}

// BindRouter accepts a Router interface and an endpoint
// in the format <proto>://<address>:<port>. It then attempts
// to bind to the endpoint.
func BindRouter(r Router, endpoint string) (net.Addr, error) {
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
	_, err = zmtpConn.Prepare(r.SecurityMechanism(), r.SocketType(), r.SocketIdentity(), true, nil)
	if err != nil {
		return netConn.LocalAddr(), err
	}

	conn := gomq.NewConnection(netConn, zmtpConn)

	r.AddConnection(conn)
	zmtpConn.Recv(r.RecvChannel())
	return netConn.LocalAddr(), nil
}

// RouteSocket is a ZMQ_ROUTER socket type.
// See: https://rfc.zeromq.org/spec:28/REQREP/
type RouterSocket struct {
	*gomq.Socket
}

// NewRouter accepts a zmtp.SecurityMechanism and an ID.
// It returns a RouterSocket as a gomq.Router interface.
func NewRouter(mechanism zmtp.SecurityMechanism, id string) Router {
	return &RouterSocket{
		Socket: gomq.NewSocket(false, zmtp.RouterSocketType, zmtp.SocketIdentity(id), mechanism),
	}
}

// Bind accepts a zeromq endpoint and binds the
// server socket to it. Currently the only transport
// supported is TCP. The endpoint string should be
// in the format "tcp://<address>:<port>".
func (r *RouterSocket) Bind(endpoint string) (net.Addr, error) {
	return BindRouter(r, endpoint)
}

type testServer struct {
	bindHost       string
	bindPort       int
	nodeID         string
	fromClient     chan message
	toClient       chan message
	routerSocket   Router
	shutdownSignal chan bool
}

func newTestServer(bindHost string, bindPort int) (server *testServer) {
	return &testServer{
		bindHost:       bindHost,
		bindPort:       bindPort,
		nodeID:         getNodeID(),
		fromClient:     make(chan message, 100),
		toClient:       make(chan message, 100),
		shutdownSignal: make(chan bool, 1),
	}
}

func (s *testServer) bind() (net.Addr, error) {
	s.routerSocket = NewRouter(zmtp.NewSecurityNull(), s.nodeID)
	go s.recv()
	go s.send()
	return BindRouter(s.routerSocket, fmt.Sprintf("tcp://%s:%d", s.bindHost, s.bindPort))
}

func (s *testServer) send() {
	for {
		select {
		case <-s.shutdownSignal:
			s.routerSocket.Close()
			return
		case msg := <-s.toClient:
			s.sendMessage(msg)
		}
	}
}

func (s *testServer) sendMessage(msg message) {
	defer func() {
		// don't panic
		err := recover()
		if err != nil {
			log.Printf("%v\n", err)
			debug.PrintStack()
		}
	}()
	serializedMessage, err := msg.serialize()
	if err != nil {
		log.Println("Msgpack encode fail:", err)
		return
	}
	err = s.routerSocket.Send(serializedMessage)
	if err != nil {
		log.Printf("Error sending to client: %v\n", err)
	}
}

func (s *testServer) recv() {
	for {
		select {
		case <-s.shutdownSignal:
			s.routerSocket.Close()
			return
		default:
			msg, err := s.routerSocket.RecvMultipart()
			if err != nil {
				log.Printf("Error reading: %v\n", err)
			} else {
				msgFromClient, err := newGenericMessageFromBytes(msg[0])
				if err != nil {
					clientReadyMessage, err2 := newClientReadyMessageFromBytes(msg[0])
					if err2 != nil {
						log.Println("Msgpack decode fail:", err)
					} else {
						// Send ack message
						data := map[string]interface{}{
							"index": 1,
						}
						clientId := clientReadyMessage.NodeID
						s.toClient <- newGenericMessage("ack", data, clientId)
						s.fromClient <- clientReadyMessage
					}
				} else {
					s.fromClient <- msgFromClient
				}
			}
		}
	}
}

func (s *testServer) close() {
	close(s.shutdownSignal)
}

func (s *testServer) start() {
	go s.bind()
}

func TestPingPong(t *testing.T) {
	masterHost := "0.0.0.0"
	rand.Seed(Now())
	masterPort := rand.Intn(1000) + 10240

	server := newTestServer(masterHost, masterPort)
	defer server.close()

	log.Printf("Starting to serve on %s:%d\n", masterHost, masterPort)
	server.start()

	time.Sleep(20 * time.Millisecond)

	// start client
	client := newClient(masterHost, masterPort, "testing ping pong")
	client.connect()
	defer client.close()

	time.Sleep(20 * time.Millisecond)

	client.sendChannel() <- newGenericMessage("ping", nil, "testing ping pong")
	msg := <-server.fromClient
	m := msg.(*genericMessage)
	if m.Type != "ping" || m.NodeID != "testing ping pong" {
		t.Error("server doesn't recv ping message")
	}

	server.toClient <- newGenericMessage("pong", nil, "testing ping pong")
	msg = <-client.recvChannel()
	m = msg.(*genericMessage)
	if m.Type != "pong" || m.NodeID != "testing ping pong" {
		t.Error("client doesn't recv pong message")
	}
}
