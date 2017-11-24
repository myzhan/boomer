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

func newClient() client {
	log.Println("Boomer is built with goczmq support.")
	var client client
	var message string
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
