package main

import (
	"log"
	"time"

	"github.com/myzhan/boomer"
)

func foo() {

	start := boomer.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := boomer.Now() - start

	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	boomer.RecordSuccess("http", "foo", elapsed, int64(10))
}

func bar() {

	start := boomer.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := boomer.Now() - start

	// Report your test result as a failure, if you write it in python, it will looks like this
	// events.request_failure.fire(request_type="udp", name="bar", response_time=100, exception=Exception("udp error"))
	boomer.RecordFailure("udp", "bar", elapsed, "udp error")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	task1 := &boomer.Task{
		Name:   "foo",
		Weight: 10,
		Fn:     foo,
	}

	task2 := &boomer.Task{
		Name:   "bar",
		Weight: 20,
		Fn:     bar,
	}

	boomer.Run(task1, task2)
}
