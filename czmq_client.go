package boomer

import (
	"fmt"
	"github.com/zeromq/goczmq"
)


type zmqClient interface {
	recv()
	send()
}

type czmqSocketClient struct {
	push_conn *goczmq.Sock
	pull_conn *goczmq.Sock
}

func NewZmqClient(masterHost string, masterPort int) (*czmqSocketClient, error){
	tcpAddr := fmt.Sprintf("tcp://%s:%d", masterHost, masterPort)
	push_conn, err := goczmq.NewPush(tcpAddr)
	tcpAddr = fmt.Sprintf(">tcp://%s:%d", masterHost, masterPort+1)
	pull_conn, err := goczmq.NewPull(tcpAddr)
	newClient := &czmqSocketClient{
		push_conn: push_conn,
		pull_conn: pull_conn,
	}
	go newClient.recv()
	go newClient.send()
	return newClient, err
}

func (this *czmqSocketClient) recv() {
	for {
		msg, _, _ := this.pull_conn.RecvFrame()
		msgFromMaster := NewMessageFromBytes(msg)
		FromServer <- msgFromMaster
	}

}


func (this *czmqSocketClient) send() {
	for {
		select {
		case msg := <- ToServer:
			this.sendMessage(msg)
			if msg.Type == "quit"{
				DisconnectedFromServer <- true
			}
		}
	}
}


func (this *czmqSocketClient) sendMessage(msg *Message){
	this.push_conn.SendFrame(msg.Serialize(), 0)
}