// +build gomq

package boomer

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/zeromq/gomq"
	"github.com/zeromq/gomq/zmtp"
)

type zmqClient interface {
	recv()
	send()
}

type gomqSocketClient struct {
	pushSocket *gomq.Socket
	pullSocket *gomq.Socket
}

func newGomqSocket(socketType zmtp.SocketType) *gomq.Socket {
	socket := gomq.NewSocket(false, socketType, zmtp.NewSecurityNull())
	return socket
}

func getNetConn(addr string) net.Conn {
	parts := strings.Split(addr, "://")
	netConn, err := net.Dial(parts[0], parts[1])
	if err != nil {
		log.Fatal(err)
	}
	return netConn
}

func connectSock(socket *gomq.Socket, addr string) {
	netConn := getNetConn(addr)
	zmtpConn := zmtp.NewConnection(netConn)
	_, err := zmtpConn.Prepare(socket.SecurityMechanism(), socket.SocketType(), false, nil)
	if err != nil {
		log.Fatal(err)
	}
	conn := gomq.NewConnection(netConn, zmtpConn)
	socket.AddConnection(conn)
	zmtpConn.Recv(socket.RecvChannel())
}

func newZmqClient(masterHost string, masterPort int) *gomqSocketClient {
	pushAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort)
	pullAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort+1)

	pushSocket := newGomqSocket(zmtp.PushSocketType)
	connectSock(pushSocket, pushAddr)

	pullSocket := newGomqSocket(zmtp.PullSocketType)
	connectSock(pullSocket, pullAddr)

	log.Println("ZMQ sockets connected")

	newClient := &gomqSocketClient{
		pushSocket: pushSocket,
		pullSocket: pullSocket,
	}
	go newClient.recv()
	go newClient.send()
	return newClient
}

func (c *gomqSocketClient) recv() {
	for {
		msg, err := c.pullSocket.Recv()
		if err != nil {
			log.Println("Error reading: %v", err)
		} else {
			msgFromMaster := newMessageFromBytes(msg)
			fromServer <- msgFromMaster
		}
	}

}

func (c *gomqSocketClient) send() {
	for {
		select {
		case msg := <-toServer:
			c.sendMessage(msg)
			if msg.Type == "quit" {
				disconnectedFromServer <- true
			}
		}
	}
}

func (c *gomqSocketClient) sendMessage(msg *message) {
	err := c.pushSocket.Send(msg.serialize())
	if err != nil {
		log.Println("Error sending: %v", err)
	}
}
