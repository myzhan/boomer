package main

import (
	"log"
	"time"

	"github.com/myzhan/boomer"
)

// This is an example about how to subscribe to boomer's internal events.

func foo() {
	start := boomer.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := boomer.Now() - start

	boomer.RecordSuccess("http", "foo", elapsed, int64(10))
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	boomer.Events.Subscribe("boomer:hatch", func(workers, hatchRate int) {
		log.Println("The master asks me to spawn", workers, "goroutines with a hatch rate of", hatchRate, "per second.")
	})

	boomer.Events.Subscribe("boomer:stop", func() {
		log.Println("The master asks me to stop.")
	})

	boomer.Events.Subscribe("boomer:quit", func() {
		log.Println("Boomer is quitting now, may be the master asks it to do so, or it receives one of SIGINT and SIGTERM.")
	})

	task := &boomer.Task{
		Name:   "foo",
		Weight: 10,
		Fn:     foo,
	}

	boomer.Run(task)
}
