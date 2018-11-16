// +build goczmq

package boomer

import (
	"fmt"
	"log"

	"github.com/zeromq/goczmq"
)

type czmqSocketClient struct {
	pushConn *goczmq.Sock
	pullConn *goczmq.Sock
}

func newClient() (client *czmqSocketClient) {
	log.Println("Boomer is built with goczmq support.")
	client = newZmqClient(masterHost, masterPort)
	log.Printf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.\n", masterHost, masterPort, masterPort+1)
	return client
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
		fromMaster <- msgFromMaster
	}
}

func (c *czmqSocketClient) send() {
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

func (c *czmqSocketClient) sendMessage(msg *message) {
	c.pushConn.SendFrame(msg.serialize(), 0)
}
