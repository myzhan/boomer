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
	pushSocket     *gomq.PushSocket
	pullSocket     *gomq.PullSocket
	shutdownSignal chan bool
}

func newClient() (client *gomqSocketClient) {
	log.Println("Boomer is built with gomq support.")
	client = newZmqClient(masterHost, masterPort)
	log.Printf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.\n", masterHost, masterPort, masterPort+1)
	return client
}

func newZmqClient(masterHost string, masterPort int) *gomqSocketClient {
	pushAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort)
	pullAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort+1)

	pushSocket := gomq.NewPush(zmtp.NewSecurityNull())
	pullSocket := gomq.NewPull(zmtp.NewSecurityNull())

	pushSocket.Connect(pushAddr)
	pullSocket.Connect(pullAddr)

	newClient := &gomqSocketClient{
		pushSocket:     pushSocket,
		pullSocket:     pullSocket,
		shutdownSignal: make(chan bool, 1),
	}
	go newClient.recv()
	go newClient.send()
	return newClient
}

func (c *gomqSocketClient) close() {
	close(c.shutdownSignal)
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
			fromMaster <- msgFromMaster
		}
	}
}

func (c *gomqSocketClient) send() {
	for {
		select {
		case <-c.shutdownSignal:
			return
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
