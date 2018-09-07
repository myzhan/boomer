package main

import (
	"flag"
	"log"
	"net"
	"time"

	"github.com/myzhan/boomer"
)

//                            +------------+
//                            |            |
//                            |   locust   |
//                            |            |
//                            +------+-----+
//                                   ^
//                                   |  qps & timeout
//                                   |
//	+------------+        +------+-----+        +------------+
//	|            |request |            |request |            |
//	| udpcopy    +------->+ udp proxy  +------->+ backend    |
//	|            |        |            |        |            |
//	+------------+        +------------+        +------------+

// While requests from udpcopy passing through this udp server, it keeps track of qps and timeout.
// Also, it can multi-copy the original request for more stress.

// Known Issues:
// 1. Once locust start the test, it can't be stopped by locust. You should restart locust and this udp server if you
//    want to restart.

// See also:
// udpcopy: https://github.com/wangbin579/udpcopy

func sendReq(req []byte, addr string) {

	a, err := net.ResolveUDPAddr("udp", addr)

	conn, err := net.DialUDP("udp", nil, a)
	if err != nil {
		boomer.Events.Publish("request_failure", "udp-dial", name, 0.0, err.Error())
		return
	}

	for n := 0; n < *number; n++ {

		startTime := boomer.Now()

		_, err = conn.Write(req)
		if err != nil {
			boomer.Events.Publish("request_failure", "udp-write", name, 0.0, err.Error())
			return
		}

		resp := make([]byte, *udpBufferSize)
		conn.SetReadDeadline(time.Now().Add(backendTimeout))

		respLength, err := conn.Read(resp)
		if err != nil {
			boomer.Events.Publish("request_failure", "udp-read", name, 0.0, err.Error())
			return
		}

		elapsed := boomer.Now() - startTime

		boomer.Events.Publish("request_success", "udp-resp", name, elapsed, int64(respLength))
	}

}

func proxy() {

	listener, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(*proxyHost),
		Port: *proxyPort,
	})
	if err != nil {
		log.Fatal("error binding on port:", *proxyPort, err)
	}

	for {
		data := make([]byte, *udpBufferSize)
		n, _, err := listener.ReadFromUDP(data)
		if err != nil {
			log.Println("error during read, current request is dropped.", err)
			continue
		}
		if *udpBufferSize <= n {
			log.Printf("request size is larger than %d，please enlarge udp-buffer-size. current request is dropped.\n", *udpBufferSize)
			continue
		}
		if !testStarted {
			// test is not started, drop current request.
			continue
		}
		go sendReq(data[:n], *backendAddr)
	}
}

func deadend() {

	testStarted = true

	for {
		time.Sleep(time.Second * 100)
	}
}

func main() {

	task := &boomer.Task{
		Name:   "udproxy",
		Weight: 10,
		Fn:     deadend,
	}

	go proxy()

	boomer.Run(task)
}

const name = "udproxy"

var testStarted = false

var backendAddr *string
var backendTimeout time.Duration
var proxyHost *string
var proxyPort *int
var udpBufferSize *int
var number *int

func init() {

	backendAddr = flag.String("backend-addr", "127.0.0.1:44444", "backend address")
	timeout := flag.Int("backend-timeout", 1000, "backend timeout(ms)")
	backendTimeout = time.Duration(*timeout) * time.Millisecond
	proxyHost = flag.String("proxy-host", "0.0.0.0", "proxy bind-host")
	proxyPort = flag.Int("proxy-port", 23333, "proxy bind-port")
	udpBufferSize = flag.Int("udp-buffer-size", 10240, "udp recv buffer size")
	number = flag.Int("number", 1, "the number of replication for multi-copying")
	flag.Parse()

}
