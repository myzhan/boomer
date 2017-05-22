// +build zeromq

package boomer

import (
	"fmt"
	"github.com/zeromq/goczmq"
	"log"
)

type zmqClient interface {
	recv()
	send()
}

type czmqSocketClient struct {
	pushConn *goczmq.Sock
	pullConn *goczmq.Sock
}

func newZmqClient(masterHost string, masterPort int) *czmqSocketClient {
	tcpAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort)
	pushConn, err := goczmq.NewPush(tcpAddr)
	if err != nil {
		log.Fatalf("Failed to create zeromq pusher, %s", err)
	}
	tcpAddr = fmt.Sprintf(">tcp://%s:%d", masterHost, masterPort+1)
	pullConn, err := goczmq.NewPull(tcpAddr)
	if err != nil {
		log.Fatalf("Failed to create zeromq puller, %s", err)
	}
	log.Println("ZMQ sockets connected")
	newClient := &czmqSocketClient{
		pushConn: pushConn,
		pullConn: pullConn,
	}
	go newClient.recv()
	go newClient.send()
	return newClient
}

func (c *czmqSocketClient) recv() {
	for {
		msg, _, _ := c.pullConn.RecvFrame()
		msgFromMaster := newMessageFromBytes(msg)
		fromServer <- msgFromMaster
	}

}

func (c *czmqSocketClient) send() {
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

func (c *czmqSocketClient) sendMessage(msg *message) {
	c.pushConn.SendFrame(msg.serialize(), 0)
}
