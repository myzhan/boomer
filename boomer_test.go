package boomer

import (
	"math"
	"os"
	"testing"
	"time"
)

func TestInitBoomer(t *testing.T) {
	initBoomer()
	defer Events.Unsubscribe("request_success", legacySuccessHandler)
	defer Events.Unsubscribe("request_failure", legacyFailureHandler)

	defer func() {
		err := recover()
		if err == nil {
			t.Error("It should panic if initBoomer is called more than once.")
		}
	}()
	initBoomer()
}

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
	if stableRateLimiter, ok := rateLimiter.(*stableRateLimiter); !ok {
		t.Error("Expected stableRateLimiter")
	} else {
		if stableRateLimiter.threshold != 100 {
			t.Error("threshold should be equals to math.MaxInt64, was", stableRateLimiter.threshold)
		}
	}

	rateLimiter, _ = createRateLimiter(0, "1")
	if rampUpRateLimiter, ok := rateLimiter.(*rampUpRateLimiter); !ok {
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
	if rampUpRateLimiter, ok := rateLimiter.(*rampUpRateLimiter); !ok {
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

func TestStartMemoryProfile(t *testing.T) {
	if _, err := os.Stat("mem.pprof"); os.IsExist(err) {
		os.Remove("mem.pprof")
	}
	startMemoryProfile("mem.pprof", 2*time.Second)
	time.Sleep(2100 * time.Millisecond)
	if _, err := os.Stat("mem.pprof"); os.IsNotExist(err) {
		t.Error("File mem.pprof is not generated")
	} else {
		os.Remove("mem.pprof")
	}
}

func TestStartCPUProfile(t *testing.T) {
	if _, err := os.Stat("cpu.pprof"); os.IsExist(err) {
		os.Remove("cpu.pprof")
	}
	startCPUProfile("cpu.pprof", 2*time.Second)
	time.Sleep(2100 * time.Millisecond)
	if _, err := os.Stat("cpu.pprof"); os.IsNotExist(err) {
		t.Error("File cpu.pprof is not generated")
	} else {
		os.Remove("cpu.pprof")
	}
}
