package boomer

import (
	"testing"
	"time"
)

func TestStableRateLimiter(t *testing.T) {
	rateLimiter := newStableRateLimiter(1, 10*time.Millisecond)
	rateLimiter.start()
	blocked := rateLimiter.acquire()
	if blocked {
		t.Error("Unexpected blocked by rate limiter")
	}
	blocked = rateLimiter.acquire()
	if !blocked {
		t.Error("Should be blocked")
	}
	rateLimiter.stop()
}

func TestWarmUpRateLimiter(t *testing.T) {
	rateLimiter := newWarmUpRateLimiter(100, "10/1s", 100*time.Millisecond)
	rateLimiter.start()
	time.Sleep(time.Second)

	for i := 0; i < 10; i++ {
		blocked := rateLimiter.acquire()
		if blocked {
			t.Error("Unexpected blocked by rate limiter")
		}
	}
	blocked := rateLimiter.acquire()
	if !blocked {
		t.Error("Should be blocked")
	}

	time.Sleep(time.Second)

	// now, the threshold is 20
	for i := 0; i < 20; i++ {
		blocked := rateLimiter.acquire()
		if blocked {
			t.Error("Unexpected blocked by rate limiter")
		}
	}
	blocked = rateLimiter.acquire()
	if !blocked {
		t.Error("Should be blocked")
	}

	rateLimiter.stop()
}

func TestParseWarmUpRate(t *testing.T) {
	rateLimiter := &warmUpRateLimiter{}
	warmUpStep, warmUpPeroid := rateLimiter.parseWarmUpRate("100")
	if warmUpStep != 100 {
		t.Error("Wrong warmUpStep, expected: 100, was:", warmUpStep)
	}
	if warmUpPeroid != time.Second {
		t.Error("Wrong warmUpPeroid, expected: 1s, was:", warmUpPeroid)
	}
	warmUpStep, warmUpPeroid = rateLimiter.parseWarmUpRate("200/10s")
	if warmUpStep != 200 {
		t.Error("Wrong warmUpStep, expected: 200, was:", warmUpStep)
	}
	if warmUpPeroid != 10*time.Second {
		t.Error("Wrong warmUpPeroid, expected: 10s, was:", warmUpPeroid)
	}
}
