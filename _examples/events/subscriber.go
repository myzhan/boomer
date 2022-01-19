package main

import (
	"log"
	"time"

	"github.com/myzhan/boomer"
)

// This is an example about how to subscribe to boomer's internal events.

func foo() {
	start := time.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := time.Since(start)

	boomer.RecordSuccess("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), int64(10))
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	boomer.Events.Subscribe(EVENT_SPAWN, func(workers int, spawnRate float64) {
		log.Println("The master asks me to spawn", workers, "goroutines with a spawn rate of", spawnRate, "per second.")
	})

	boomer.Events.Subscribe(EVENT_STOP, func() {
		log.Println("The master asks me to stop.")
	})

	boomer.Events.Subscribe(EVENT_QUIT, func() {
		log.Println("Boomer is quitting now, may be the master asks it to do so, or it receives one of SIGINT and SIGTERM.")
	})

	task := &boomer.Task{
		Name:   "foo",
		Weight: 10,
		Fn:     foo,
	}

	boomer.Run(task)
}
