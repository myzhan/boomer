package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/myzhan/boomer"
)

func foo() {
	start := time.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := time.Since(start)

	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	globalBoomer.RecordSuccess("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), int64(10))
}

func waitForQuit() {
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		globalBoomer.Quit()
		wg.Done()
	}()

	wg.Wait()
}

var globalBoomer = boomer.NewBoomer("127.0.0.1", 5557)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	task1 := &boomer.Task{
		Name:   "foo",
		Weight: 10,
		Fn:     foo,
	}

	ratelimiter, _ := boomer.NewRampUpRateLimiter(1000, "100/1s", time.Second)
	log.Println("the max rps is limited to 1000/s, with a rampup rate 100/1s.")
	globalBoomer.SetRateLimiter(ratelimiter)

	globalBoomer.Run(task1)

	waitForQuit()
	log.Println("shut down")
}
