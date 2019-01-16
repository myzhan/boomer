package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/myzhan/boomer"
)

var bindHost string
var bindPort string
var stopChannel chan bool

func worker() {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", bindHost, bindPort))
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	readBuff := make([]byte, 5)

	// Usually, you shouldn't run an infinite loop in worker function, unless you know exactly what you are doing.
	// It will disable features like rate limit.
	for {
		select {
		case <-stopChannel:
			return
		default:
			// timeout after 1 second
			start := boomer.Now()
			conn.SetWriteDeadline(time.Now().Add(time.Second))
			n, err := conn.Write([]byte("hello"))
			elapsed := boomer.Now() - start
			if err != nil {
				boomer.RecordFailure("tcp", "write failure", elapsed, err.Error())
				continue
			}
			// len("hello") == 5
			if n != 5 {
				boomer.RecordFailure("tcp", "write mismatch", elapsed, "write mismatch")
				continue
			}

			conn.SetReadDeadline(time.Now().Add(time.Second))
			n, err = conn.Read(readBuff)
			elapsed = boomer.Now() - start
			if err != nil {
				boomer.RecordFailure("tcp", "read failure", elapsed, err.Error())
				continue
			}

			if n != 5 {
				boomer.RecordFailure("tcp", "read mismatch", elapsed, "read mismatch")
				continue
			}

			boomer.RecordSuccess("tcp", "success", elapsed, 5)
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.Parse()

	task := &boomer.Task{
		Name:   "tcp",
		Weight: 10,
		Fn:     worker,
	}

	boomer.Events.Subscribe("boomer:hatch", func(workers, hatchRate int) {
		stopChannel = make(chan bool)
	})

	boomer.Events.Subscribe("boomer:stop", func() {
		close(stopChannel)
	})

	boomer.Events.Subscribe("boomer:quit", func() {
		close(stopChannel)
		time.Sleep(time.Second)
	})

	boomer.Run(task)
}

func init() {
	flag.StringVar(&bindHost, "host", "127.0.0.1", "host")
	flag.StringVar(&bindPort, "port", "4567", "port")
}
