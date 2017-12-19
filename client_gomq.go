// +build !goczmq

package boomer

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/myzhan/boomer/gomq"
	"github.com/myzhan/boomer/gomq/zmtp"
)

type gomqSocketClient struct {
	pushSocket *gomq.Socket
	pullSocket *gomq.Socket
}

func newClient() client {
	log.Println("Boomer is built with gomq support.")
	var message string
	var client client
	if *rpc == "zeromq" {
		client = newZmqClient(*masterHost, *masterPort)
		message = fmt.Sprintf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.", *masterHost, *masterPort, *masterPort+1)
	} else if *rpc == "socket" {
		client = newSocketClient(*masterHost, *masterPort)
		message = fmt.Sprintf("Boomer is connected to master(%s:%d) press Ctrl+c to quit.", *masterHost, *masterPort)
	} else {
		log.Fatal("Unknown rpc type:", *rpc)
	}
	log.Println(message)
	return client
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
			log.Printf("Error reading: %v\n", err)
		} else {
			msgFromMaster := newMessageFromBytes(msg)
			fromMaster <- msgFromMaster
		}
	}

}

func (c *gomqSocketClient) send() {
	for {
		select {
		case msg := <-toMaster:
			c.sendMessage(msg)
			if msg.Type == "quit" {
				disconnectedFromMaster <- true
			}
		}
	}
}

func (c *gomqSocketClient) sendMessage(msg *message) {
	err := c.pushSocket.Send(msg.serialize())
	if err != nil {
		log.Printf("Error sending: %v\n", err)
	}
}
