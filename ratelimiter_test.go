package boomer

import (
	"testing"
	"time"
)

func TestStableRateLimiter(t *testing.T) {
	rateLimiter := NewStableRateLimiter(1, 10*time.Millisecond)
	rateLimiter.Start()
	defer rateLimiter.Stop()

	blocked := rateLimiter.Acquire()
	if blocked {
		t.Error("Unexpected blocked by rate limiter")
	}
	blocked = rateLimiter.Acquire()
	if !blocked {
		t.Error("Should be blocked")
	}
}

func TestRampUpRateLimiter(t *testing.T) {
	rateLimiter, _ := NewRampUpRateLimiter(100, "10/200ms", 100*time.Millisecond)
	rateLimiter.Start()
	defer rateLimiter.Stop()

	time.Sleep(110 * time.Millisecond) // fill 10 in 100ms, the bucket is now 10

	for i := 0; i < 10; i++ { // acquire 10, the bucket will become 0
		blocked := rateLimiter.Acquire() // -1
		if blocked {
			t.Error("Unexpected blocked by rate limiter")
		}
	}
	blocked := rateLimiter.Acquire() // bucket is empty, should block
	if !blocked {
		t.Error("Should be blocked")
	}

	time.Sleep(190 * time.Millisecond) // fill 20 in 200ms, the bucket is now 20

	for i := 0; i < 20; i++ { // acquire 20, the bucket will become 0
		blocked := rateLimiter.Acquire()
		if blocked {
			t.Error("Unexpected blocked by rate limiter")
		}
	}
	blocked = rateLimiter.Acquire() // bucket is empty, should block
	if !blocked {
		t.Error("Should be blocked")
	}
}

func TestParseRampUpRate(t *testing.T) {
	rateLimiter := &RampUpRateLimiter{}
	rampUpStep, rampUpPeriod, _ := rateLimiter.parseRampUpRate("100")
	if rampUpStep != 100 {
		t.Error("Wrong rampUpStep, expected: 100, was:", rampUpStep)
	}
	if rampUpPeriod != time.Second {
		t.Error("Wrong rampUpPeriod, expected: 1s, was:", rampUpPeriod)
	}
	rampUpStep, rampUpPeriod, _ = rateLimiter.parseRampUpRate("200/10s")
	if rampUpStep != 200 {
		t.Error("Wrong rampUpStep, expected: 200, was:", rampUpStep)
	}
	if rampUpPeriod != 10*time.Second {
		t.Error("Wrong rampUpPeriod, expected: 10s, was:", rampUpPeriod)
	}
}

func TestParseInvalidRampUpRate(t *testing.T) {
	rateLimiter := &RampUpRateLimiter{}

	_, _, err := rateLimiter.parseRampUpRate("A/1m")
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}

	_, _, err = rateLimiter.parseRampUpRate("A")
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}

	_, _, err = rateLimiter.parseRampUpRate("200/1s/")
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}

	_, _, err = rateLimiter.parseRampUpRate("200/1")
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}

	rateLimiter, err = NewRampUpRateLimiter(1, "200/1", time.Second)
	if err == nil || err != ErrParsingRampUpRate {
		t.Error("Expected ErrParsingRampUpRate")
	}
}
