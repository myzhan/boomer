package main

import (
	"flag"
	"fmt"
	"log"
	"net"
)

var bindHost string
var bindPort string

// EchoServer will return a fixed size message(5 bytes) sent by the client
type EchoServer struct {
	bindHost string
	bindPort string
}

func newEchoServer(bindHost, bindPort string) *EchoServer {
	return &EchoServer{
		bindHost: bindHost,
		bindPort: bindPort,
	}
}

func (s *EchoServer) start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", bindHost, bindPort))
	if err != nil {
		log.Fatalln(err)
	}
	defer listener.Close()

	log.Println("Start serving on", fmt.Sprintf("%s:%s", bindHost, bindPort))

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accept", err)
		} else {
			go s.handleConnection(conn)
		}
	}
}

func (s *EchoServer) handleConnection(conn net.Conn) {
	for {
		readBuff := make([]byte, 5)
		n, err := conn.Read(readBuff)
		if err != nil {
			log.Println(err)
			return
		}
		// len("hello") == 5
		if n != 5 {
			log.Println("Recv", n, "bytes")
			return
		}
		conn.Write(readBuff)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.Parse()
	server := newEchoServer(bindHost, bindPort)
	server.start()
}

func init() {
	flag.StringVar(&bindHost, "host", "127.0.0.1", "host")
	flag.StringVar(&bindPort, "port", "4567", "port")
}
