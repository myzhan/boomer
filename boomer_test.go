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
	defer defaultStats.close()

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
	runTasks = "increaseCount,foobar"
	runTasksForTest(taskA)

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
	if warmUpRateLimiter, ok := rateLimiter.(*warmUpRateLimiter); !ok {
		t.Error("Expected warmUpRateLimiter")
	} else {
		if warmUpRateLimiter.maxThreshold != math.MaxInt64 {
			t.Error("maxThreshold should be equals to math.MaxInt64, was", warmUpRateLimiter.maxThreshold)
		}
		if warmUpRateLimiter.warmUpRate != "1" {
			t.Error("warmUpRate should be equals to \"1\", was", warmUpRateLimiter.warmUpRate)
		}
	}

	rateLimiter, _ = createRateLimiter(10, "2/2s")
	if warmUpRateLimiter, ok := rateLimiter.(*warmUpRateLimiter); !ok {
		t.Error("Expected warmUpRateLimiter")
	} else {
		if warmUpRateLimiter.maxThreshold != 10 {
			t.Error("maxThreshold should be equals to 10, was", warmUpRateLimiter.maxThreshold)
		}
		if warmUpRateLimiter.warmUpRate != "2/2s" {
			t.Error("warmUpRate should be equals to \"2/2s\", was", warmUpRateLimiter.warmUpRate)
		}
		if warmUpRateLimiter.warmUpStep != 2 {
			t.Error("warmUpStep should be equals to 2, was", warmUpRateLimiter.warmUpStep)
		}
		if warmUpRateLimiter.warmUpPeroid != 2*time.Second {
			t.Error("warmUpPeroid should be equals to 2 seconds, was", warmUpRateLimiter.warmUpPeroid)
		}
	}
}

func TestStartMemoryProfile(t *testing.T) {
	if _, err := os.Stat("mem.pprof"); os.IsExist(err) {
		os.Remove("mem.pprof")
	}
	startMemoryProfile("mem.pprof", 3*time.Second)
	time.Sleep(4 * time.Second)
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
	startCPUProfile("cpu.pprof", 3*time.Second)
	time.Sleep(4 * time.Second)
	if _, err := os.Stat("cpu.pprof"); os.IsNotExist(err) {
		t.Error("File cpu.pprof is not generated")
	} else {
		os.Remove("cpu.pprof")
	}
}
