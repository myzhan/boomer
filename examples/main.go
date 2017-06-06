package main

import "github.com/myzhan/boomer"
import "time"

func foo() {
	time.Sleep(100 * time.Millisecond)
	/*
	   Report your test result as a success, if you write it in python, it will looks like this
	   events.request_success.fire(request_type="http", name="foo", response_time=100.0, response_length=10)
	*/
	boomer.Events.Publish("request_success", "foo", "http", 100.0, int64(10))
}

func bar() {
	time.Sleep(100 * time.Millisecond)
	/*
	   Report your test result as a failure, if you write it in python, it will looks like this
	   events.request_failure.fire(request_type="udp", name="bar", response_time=100.0, exception=Exception("udp error"))
	*/
	boomer.Events.Publish("request_failure", "bar", "udp", 100.0, "udp error")
}

func main() {

	task1 := &boomer.Task{
		Name: "foo",
		Weight: 10,
		Fn:     foo,
	}

	task2 := &boomer.Task{
		Name: "bar",
		Weight: 20,
		Fn:     bar,
	}

	boomer.Run(task1, task2)

}
