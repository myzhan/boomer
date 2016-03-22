// +build zeromq

package boomer

import (
	"log"
	"fmt"
	"github.com/zeromq/goczmq"
)


type zmqClient interface {
	recv()
	send()
}

type czmqSocketClient struct {
	pushConn *goczmq.Sock
	pullConn *goczmq.Sock
}

func NewZmqClient(masterHost string, masterPort int) (*czmqSocketClient) {
	tcpAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort)
	pushConn, err := goczmq.NewPush(tcpAddr)
	if err != nil {
		log.Fatalf("Failed to create zeromq pusher", err)
	}
	tcpAddr = fmt.Sprintf(">tcp://%s:%d", masterHost, masterPort + 1)
	pullConn, err := goczmq.NewPull(tcpAddr)
	if err != nil {
		log.Fatalf("Failed to create zeromq puller", err)
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

func (this *czmqSocketClient) recv() {
	for {
		msg, _, _ := this.pullConn.RecvFrame()
		msgFromMaster := NewMessageFromBytes(msg)
		FromServer <- msgFromMaster
	}

}


func (this *czmqSocketClient) send() {
	for {
		select {
		case msg := <-ToServer:
			this.sendMessage(msg)
			if msg.Type == "quit" {
				DisconnectedFromServer <- true
			}
		}
	}
}


func (this *czmqSocketClient) sendMessage(msg *Message) {
	this.pushConn.SendFrame(msg.Serialize(), 0)
}