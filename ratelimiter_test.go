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

func TestRampUpRateLimiter(t *testing.T) {
	rateLimiter, _ := newRampUpRateLimiter(100, "10/200ms", 100*time.Millisecond)
	rateLimiter.start()
	time.Sleep(110 * time.Millisecond)

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

	time.Sleep(110 * time.Millisecond)

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

func TestParseRampUpRate(t *testing.T) {
	rateLimiter := &rampUpRateLimiter{}
	rampUpStep, rampUpPeroid, _ := rateLimiter.parseRampUpRate("100")
	if rampUpStep != 100 {
		t.Error("Wrong rampUpStep, expected: 100, was:", rampUpStep)
	}
	if rampUpPeroid != time.Second {
		t.Error("Wrong rampUpPeroid, expected: 1s, was:", rampUpPeroid)
	}
	rampUpStep, rampUpPeroid, _ = rateLimiter.parseRampUpRate("200/10s")
	if rampUpStep != 200 {
		t.Error("Wrong rampUpStep, expected: 200, was:", rampUpStep)
	}
	if rampUpPeroid != 10*time.Second {
		t.Error("Wrong rampUpPeroid, expected: 10s, was:", rampUpPeroid)
	}

	rampUpStep, rampUpPeroid, err := rateLimiter.parseRampUpRate("200/1s/")
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}

	rampUpStep, rampUpPeroid, err = rateLimiter.parseRampUpRate("200/1")
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}
}
