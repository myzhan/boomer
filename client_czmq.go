// +build goczmq

package boomer

import (
	"fmt"
	"log"

	"github.com/zeromq/goczmq"
)

type czmqSocketClient struct {
	masterHost string
	pushPort   int
	pullPort   int

	pushConn       *goczmq.Sock
	pullConn       *goczmq.Sock
	shutdownSignal chan bool

	fromMaster             chan *message
	toMaster               chan *message
	disconnectedFromMaster chan bool
}

func newClient(masterHost string, masterPort int) (client *czmqSocketClient) {
	log.Println("Boomer is built with goczmq support.")
	client = &czmqSocketClient{
		masterHost:             masterHost,
		pushPort:               masterPort,
		pullPort:               masterPort + 1,
		shutdownSignal:         make(chan bool),
		fromMaster:             make(chan *message, 100),
		toMaster:               make(chan *message, 100),
		disconnectedFromMaster: make(chan bool),
	}

	return client
}

func (c *czmqSocketClient) connect() {
	tcpAddr := fmt.Sprintf("tcp://%s:%d", c.masterHost, c.pushPort)
	pushConn, err := goczmq.NewPush(tcpAddr)
	if err != nil {
		log.Fatalf("Failed to create zeromq pusher, %s", err)
	}
	c.pushConn = pushConn

	tcpAddr = fmt.Sprintf(">tcp://%s:%d", c.masterHost, c.pullPort)
	pullConn, err := goczmq.NewPull(tcpAddr)
	if err != nil {
		log.Fatalf("Failed to create zeromq puller, %s", err)
	}
	c.pullConn = pullConn

	log.Printf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.\n", c.masterHost, c.pushPort, c.pullPort)

	go c.recv()
	go c.send()
}

func (c *czmqSocketClient) close() {
	close(c.shutdownSignal)
}

func (c *czmqSocketClient) recvChannel() chan *message {
	return c.fromMaster
}

func (c *czmqSocketClient) recv() {
	for {
		select {
		case <-c.shutdownSignal:
			return
		default:
			msg, _, _ := c.pullConn.RecvFrame()
			msgFromMaster := newMessageFromBytes(msg)
			c.fromMaster <- msgFromMaster
		}
	}
}

func (c *czmqSocketClient) sendChannel() chan *message {
	return c.toMaster
}

func (c *czmqSocketClient) send() {
	for {
		select {
		case <-c.shutdownSignal:
			return
		case msg := <-c.toMaster:
			c.sendMessage(msg)
			if msg.Type == "quit" {
				c.disconnectedFromMaster <- true
			}
		}
	}
}

func (c *czmqSocketClient) sendMessage(msg *message) {
	c.pushConn.SendFrame(msg.serialize(), 0)
}

func (c *czmqSocketClient) disconnectedChannel() chan bool {
	return c.disconnectedFromMaster
}
