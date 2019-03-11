package boomer

import (
	"math"
	"testing"
	"time"
)

func TestRunTasksForTest(t *testing.T) {
	count := 0
	taskA := &Task{
		Name: "increaseCount",
		Fn: func() {
			count++
		},
	}
	taskWithoutName := &Task{
		Name: "",
		Fn: func() {
			count++
		},
	}
	runTasks = "increaseCount,foobar"

	runTasksForTest(taskA, taskWithoutName)

	if count != 1 {
		t.Error("count is", count, "expected: 1")
	}
}

func TestCreateRatelimiter(t *testing.T) {
	rateLimiter, _ := createRateLimiter(100, "-1")
	if stableRateLimiter, ok := rateLimiter.(*StableRateLimiter); !ok {
		t.Error("Expected stableRateLimiter")
	} else {
		if stableRateLimiter.threshold != 100 {
			t.Error("threshold should be equals to math.MaxInt64, was", stableRateLimiter.threshold)
		}
	}

	rateLimiter, _ = createRateLimiter(0, "1")
	if rampUpRateLimiter, ok := rateLimiter.(*RampUpRateLimiter); !ok {
		t.Error("Expected rampUpRateLimiter")
	} else {
		if rampUpRateLimiter.maxThreshold != math.MaxInt64 {
			t.Error("maxThreshold should be equals to math.MaxInt64, was", rampUpRateLimiter.maxThreshold)
		}
		if rampUpRateLimiter.rampUpRate != "1" {
			t.Error("rampUpRate should be equals to \"1\", was", rampUpRateLimiter.rampUpRate)
		}
	}

	rateLimiter, _ = createRateLimiter(10, "2/2s")
	if rampUpRateLimiter, ok := rateLimiter.(*RampUpRateLimiter); !ok {
		t.Error("Expected rampUpRateLimiter")
	} else {
		if rampUpRateLimiter.maxThreshold != 10 {
			t.Error("maxThreshold should be equals to 10, was", rampUpRateLimiter.maxThreshold)
		}
		if rampUpRateLimiter.rampUpRate != "2/2s" {
			t.Error("rampUpRate should be equals to \"2/2s\", was", rampUpRateLimiter.rampUpRate)
		}
		if rampUpRateLimiter.rampUpStep != 2 {
			t.Error("rampUpStep should be equals to 2, was", rampUpRateLimiter.rampUpStep)
		}
		if rampUpRateLimiter.rampUpPeroid != 2*time.Second {
			t.Error("rampUpPeroid should be equals to 2 seconds, was", rampUpRateLimiter.rampUpPeroid)
		}
	}
}

func TestRecordSuccess(t *testing.T) {
	defaultBoomer = NewBoomer("127.0.0.1", 5557)
	defaultBoomer.runner = newRunner(nil, nil, "asap")
	RecordSuccess("http", "foo", int64(1), int64(10))

	requestSuccessMsg := <-defaultBoomer.runner.stats.requestSuccessChannel
	if requestSuccessMsg.requestType != "http" {
		t.Error("Expected: http, got:", requestSuccessMsg.requestType)
	}
	if requestSuccessMsg.responseTime != int64(1) {
		t.Error("Expected: 1, got:", requestSuccessMsg.responseTime)
	}
	defaultBoomer = nil
}

func TestRecordFailure(t *testing.T) {
	defaultBoomer = NewBoomer("127.0.0.1", 5557)
	defaultBoomer.runner = newRunner(nil, nil, "asap")
	RecordFailure("udp", "bar", int64(2), "udp error")

	requestFailureMsg := <-defaultBoomer.runner.stats.requestFailureChannel
	if requestFailureMsg.requestType != "udp" {
		t.Error("Expected: udp, got:", requestFailureMsg.requestType)
	}
	if requestFailureMsg.responseTime != int64(2) {
		t.Error("Expected: 2, got:", requestFailureMsg.responseTime)
	}
	if requestFailureMsg.error != "udp error" {
		t.Error("Expected: udp error, got:", requestFailureMsg.error)
	}
	defaultBoomer = nil
}
