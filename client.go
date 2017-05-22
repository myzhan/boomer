package boomer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

type client interface {
	recv()
	send()
}

var fromServer = make(chan *message, 100)
var toServer = make(chan *message, 100)
var disconnectedFromServer = make(chan bool)

type socketClient struct {
	conn *net.TCPConn
}

func newSocketClient(masterHost string, masterPort int) *socketClient {
	serverAddr := fmt.Sprintf("%s:%d", masterHost, masterPort)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", serverAddr)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Fatalf("Failed to connect to the Locust master: %s %s", serverAddr, err)
	}
	conn.SetNoDelay(true)
	newClient := &socketClient{
		conn: conn,
	}
	go newClient.recv()
	go newClient.send()
	return newClient
}

func (c *socketClient) recvBytes(length int) []byte {
	buf := make([]byte, length)
	for length > 0 {
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		length = length - n
	}
	return buf
}

func (c *socketClient) recv() {
	for {
		h := c.recvBytes(4)
		msgLength := binary.BigEndian.Uint32(h)
		msg := c.recvBytes(int(msgLength))
		msgFromMaster := newMessageFromBytes(msg)
		fromServer <- msgFromMaster
	}

}

func (c *socketClient) send() {
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

func (c *socketClient) sendMessage(msg *message) {
	packed := msg.serialize()
	buf := new(bytes.Buffer)

	// use a fixed length header that indicates the length of the body
	// -----------------
	// | length | body |
	// -----------------

	binary.Write(buf, binary.BigEndian, int32(len(packed)))
	buf.Write(packed)
	c.conn.Write(buf.Bytes())
}
