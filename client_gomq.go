// +build !goczmq

package boomer

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/zeromq/gomq"
	"github.com/zeromq/gomq/zmtp"
)

type gomqSocketClient struct {
	masterHost string
	masterPort int
	identity   string

	dealerSocket gomq.Dealer

	fromMaster             chan *message
	toMaster               chan *message
	disconnectedFromMaster chan bool
	shutdownSignal         chan bool
}

func newClient(masterHost string, masterPort int, identity string) (client *gomqSocketClient) {
	log.Println("Boomer is built with gomq support.")
	client = &gomqSocketClient{
		masterHost:             masterHost,
		masterPort:             masterPort,
		identity:               identity,
		fromMaster:             make(chan *message, 100),
		toMaster:               make(chan *message, 100),
		disconnectedFromMaster: make(chan bool),
		shutdownSignal:         make(chan bool),
	}
	return client
}

func (c *gomqSocketClient) connect() {
	addr := fmt.Sprintf("tcp://%s:%d", c.masterHost, c.masterPort)
	c.dealerSocket = gomq.NewDealer(zmtp.NewSecurityNull(), c.identity)

	if err := c.dealerSocket.Connect(addr); err != nil {
		log.Printf("Failed to connect to master(%s) with error %v\n", addr, err)
		if strings.Contains(err.Error(), "Socket type DEALER is not compatible with PULL") {
			log.Println("Newer version of locust changes ZMQ socket to DEALER and ROUTER, you should update your locust version.")
		}
		os.Exit(1)
		return
	}

	log.Printf("Boomer is connected to master(%s) press Ctrl+c to quit.\n", addr)
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
			log.Printf("The underlying socket connected to master(%s:%d) may be broken, please restart both locust and boomer\n", c.masterHost, c.masterPort)
			runtime.Goexit()
		}
	}()
	for {
		msg, err := c.dealerSocket.Recv()
		if err != nil {
			log.Printf("Error reading: %v\n", err)
			continue
		}
		decodedMsg, err := newMessageFromBytes(msg)
		if err != nil {
			log.Printf("Msgpack decode fail: %v\n", err)
			continue
		}
		if decodedMsg.NodeID != c.identity {
			log.Printf("Recv a %s message for node(%s), not for me(%s), dropped.\n", decodedMsg.Type, decodedMsg.NodeID, c.identity)
			continue
		}
		c.fromMaster <- decodedMsg
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
	serializedMessage, err := msg.serialize()
	if err != nil {
		log.Printf("Msgpack encode fail: %v\n", err)
		return
	}
	err = c.dealerSocket.Send(serializedMessage)
	if err != nil {
		log.Printf("Error sending: %v\n", err)
	}
}

func (c *gomqSocketClient) disconnectedChannel() chan bool {
	return c.disconnectedFromMaster
}
