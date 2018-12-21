package main

import "github.com/myzhan/boomer"
import (
	"log"
	"sync/atomic"
	"time"
)

var step = int64(1)
var currentRPS = int64(0)
var currentRPSThreshold = int64(0)
var waitChannel = make(chan bool)

func coworker() {

	token := atomic.AddInt64(&currentRPSThreshold, -1)
	if token < 0 {
		<-waitChannel
		return
	}

	boomer.RecordSuccess("rps", "pacing", int64(1), int64(10))

}

func updateThreshold() {

	var ticker = time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			currentRPS = currentRPS + step
			atomic.StoreInt64(&currentRPSThreshold, currentRPS)
			log.Println("Current RPS", currentRPS)

			close(waitChannel)
			waitChannel = make(chan bool)
		}
	}

}

func main() {

	task := &boomer.Task{
		Name:   "foo",
		Weight: 10,
		Fn:     coworker,
	}

	go updateThreshold()

	boomer.Run(task)
}
