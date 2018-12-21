package main

import (
	"time"

	"github.com/myzhan/boomer"
)

// GetName returns a string assigned to task.Name
func GetName() string {
	return "foo"
}

// GetWeight returns an integer assigned to task.Weight
func GetWeight() int {
	return 10
}

// Execute is assigned to task.Fn
func Execute() {
	start := boomer.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := boomer.Now() - start

	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	boomer.RecordSuccess("plugin", "success", elapsed, int64(10))
}
