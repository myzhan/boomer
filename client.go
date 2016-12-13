package boomer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

type Client interface {
	recv()
	send()
}

var FromServer = make(chan *Message, 100)
var ToServer = make(chan *Message, 100)
var DisconnectedFromServer = make(chan bool)

type SocketClient struct {
	conn *net.TCPConn
}

func NewSocketClient(masterHost string, masterPort int) *SocketClient {
	serverAddr := fmt.Sprintf("%s:%d", masterHost, masterPort)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", serverAddr)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Fatalf("Failed to connect to the Locust master: %s %s", serverAddr, err)
	}
	conn.SetNoDelay(true)
	newClient := &SocketClient{
		conn: conn,
	}
	go newClient.recv()
	go newClient.send()
	return newClient
}

func (this *SocketClient) recvBytes(length int) []byte {
	buf := make([]byte, length)
	for length > 0 {
		n, err := this.conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		length = length - n
	}
	return buf
}

func (this *SocketClient) recv() {
	for {
		h := this.recvBytes(4)
		msgLength := binary.BigEndian.Uint32(h)
		msg := this.recvBytes(int(msgLength))
		msgFromMaster := NewMessageFromBytes(msg)
		FromServer <- msgFromMaster
	}

}

func (this *SocketClient) send() {
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

func (this *SocketClient) sendMessage(msg *Message) {
	packed := msg.Serialize()
	buf := new(bytes.Buffer)
	/*
		use a fixed length header that indicates the length of the body
		-----------------
		| length | body |
		-----------------
	*/
	binary.Write(buf, binary.BigEndian, int32(len(packed)))
	buf.Write(packed)
	this.conn.Write(buf.Bytes())
}
