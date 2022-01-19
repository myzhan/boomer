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

// See also:
// udpcopy: https://github.com/wangbin579/udpcopy

const name = "udpproxy"

var testStarted = false
var workersPool []*worker

// command line arguments
var backendAddr *string
var backendTimeout time.Duration
var proxyHost *string
var proxyPort *int
var udpBufferSize *int
var number *int
var workersCount *int
var dontRead *bool

type worker struct {
	conn     *net.UDPConn
	requests chan []byte
	recvBuff []byte
}

func newWorker(remoteAddr string) *worker {
	addr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		log.Fatalln("Failed to create worker", err)
		return nil
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatalln("Failed to create worker", err)
		return nil
	}
	requests := make(chan []byte, 1000)
	recvBuff := make([]byte, *udpBufferSize)
	go func() {
		for req := range requests {
			for n := 0; n < *number; n++ {
				startTime := time.Now()
				wn, err := conn.Write(req)
				if err != nil {
					boomer.RecordFailure(name, "udp-write", 0.0, err.Error())
					continue
				}

				if *dontRead {
					elapsed := time.Since(startTime)
					boomer.RecordSuccess(name, "udp-write", elapsed.Nanoseconds()/int64(time.Millisecond), int64(wn))
				} else {
					conn.SetReadDeadline(time.Now().Add(backendTimeout))
					respLength, err := conn.Read(recvBuff)
					if err != nil {
						boomer.RecordFailure(name, "udp-read", 0.0, err.Error())
						continue
					}
					elapsed := time.Since(startTime)
					boomer.RecordSuccess(name, "udp-resp", elapsed.Nanoseconds()/int64(time.Millisecond), int64(respLength))
				}
			}
		}
	}()

	return &worker{
		conn:     conn,
		requests: requests,
		recvBuff: recvBuff,
	}
}

func createWorkers() {
	workersPool = make([]*worker, *workersCount)
	for n := 0; n < *workersCount; n++ {
		workersPool[n] = newWorker(*backendAddr)
	}
}

func proxy() {
	// Pooling workers
	createWorkers()

	listener, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(*proxyHost),
		Port: *proxyPort,
	})
	if err != nil {
		log.Fatal("error binding on port:", *proxyPort, err)
	}

	workerIndex := uint64(0)

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

		// Round-robin selection
		workerIndex = workerIndex + 1
		selectedWorker := workersPool[workerIndex%(uint64(*workersCount))]
		selectedWorker.requests <- data[:n]
	}
}

func startTest(workers int, spawnRate float64) {
	testStarted = true
}

func stopTest() {
	testStarted = false
}

func deadend() {
	for {
		time.Sleep(time.Second * 100)
	}
}

func main() {
	boomer.Events.Subscribe("boome:spawn", startTest)
	boomer.Events.Subscribe(EVENT_STOP, stopTest)

	task := &boomer.Task{
		Name:   "udproxy",
		Weight: 10,
		Fn:     deadend,
	}

	go proxy()

	boomer.Run(task)
}

func init() {
	backendAddr = flag.String("backend-addr", "127.0.0.1:44444", "backend address")
	timeout := flag.Int("backend-timeout", 1000, "backend timeout(ms)")
	proxyHost = flag.String("proxy-host", "0.0.0.0", "proxy bind-host")
	proxyPort = flag.Int("proxy-port", 23333, "proxy bind-port")
	workersCount = flag.Int("workers", 20, "UDP workers")
	udpBufferSize = flag.Int("udp-buffer-size", 4096, "udp recv buffer size")
	number = flag.Int("number", 1, "the number of replication for multi-copying")
	dontRead = flag.Bool("dontread", false, "do not wait for backend's response")
	flag.Parse()

	backendTimeout = time.Duration(*timeout) * time.Millisecond
}
