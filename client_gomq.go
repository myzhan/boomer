// +build !goczmq

package boomer

import (
	"fmt"
	"log"
	"runtime"
	"runtime/debug"

	"github.com/zeromq/gomq"
	"github.com/zeromq/gomq/zmtp"
)

type gomqSocketClient struct {
	masterHost string
	pushPort   int
	pullPort   int

	pushSocket     *gomq.PushSocket
	pullSocket     *gomq.PullSocket
	shutdownSignal chan bool

	fromMaster             chan *message
	toMaster               chan *message
	disconnectedFromMaster chan bool
}

func newClient(masterHost string, masterPort int) (client *gomqSocketClient) {
	log.Println("Boomer is built with gomq support.")
	client = &gomqSocketClient{
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

func (c *gomqSocketClient) connect() {
	pushAddr := fmt.Sprintf("tcp://%s:%d", c.masterHost, c.pushPort)
	pullAddr := fmt.Sprintf("tcp://%s:%d", c.masterHost, c.pullPort)

	pushSocket := gomq.NewPush(zmtp.NewSecurityNull())
	c.pushSocket = pushSocket
	pullSocket := gomq.NewPull(zmtp.NewSecurityNull())
	c.pullSocket = pullSocket

	c.pushSocket.Connect(pushAddr)
	c.pullSocket.Connect(pullAddr)
	log.Printf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.\n", masterHost, masterPort, masterPort+1)
	go c.recv()
	go c.send()
}

func (c *gomqSocketClient) close() {
	close(c.shutdownSignal)
}

func (c *gomqSocketClient) recvChannel() chan *message {
	return c.fromMaster
}

func (c *gomqSocketClient) recv() {
	defer func() {
		// Temporary work around for https://github.com/zeromq/gomq/issues/75
		err := recover()
		if err != nil {
			log.Printf("%v\n", err)
			debug.PrintStack()
			log.Printf("The underlying socket connected to master(%s:%d) may be broken, please restart both locust and boomer\n", masterHost, masterPort+1)
			runtime.Goexit()
		}
	}()
	for {
		msg, err := c.pullSocket.Recv()
		if err != nil {
			log.Printf("Error reading: %v\n", err)
		} else {
			msgFromMaster := newMessageFromBytes(msg)
			c.fromMaster <- msgFromMaster
		}
	}
}

func (c *gomqSocketClient) sendChannel() chan *message {
	return c.toMaster
}

func (c *gomqSocketClient) send() {
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

func (c *gomqSocketClient) sendMessage(msg *message) {
	err := c.pushSocket.Send(msg.serialize())
	if err != nil {
		log.Printf("Error sending: %v\n", err)
	}
}

func (c *gomqSocketClient) disconnectedChannel() chan bool {
	return c.disconnectedFromMaster
}
