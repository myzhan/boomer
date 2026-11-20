package boomer

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test ratelimiter", func() {

	It("test stable ratelimiter", func() {
		rateLimiter := NewStableRateLimiter(1, 10*time.Millisecond)
		rateLimiter.Start()
		defer rateLimiter.Stop()

		Expect(rateLimiter.Acquire()).NotTo(BeTrue())
		Expect(rateLimiter.Acquire()).To(BeTrue())
	})

	It("test stable ratelimiter start and stop many times", func() {
		Expect(func() {
			ratelimiter := NewStableRateLimiter(100, 5*time.Millisecond)
			for i := 0; i < 500; i++ {
				ratelimiter.Start()
				time.Sleep(5 * time.Millisecond)
				ratelimiter.Stop()
			}
		}).Should(Not(Panic()))
	})

	It("test rampup ratelimiter", func() {
		rateLimiter, _ := NewRampUpRateLimiter(100, "10/200ms", 100*time.Millisecond)
		rateLimiter.Start()
		defer rateLimiter.Stop()

		for i := 0; i < 10; i++ {
			Expect(rateLimiter.Acquire()).NotTo(BeTrue())
		}
		Expect(rateLimiter.Acquire()).To(BeTrue())

		time.Sleep(210 * time.Millisecond)

		// now, the threshold is 20
		for i := 0; i < 20; i++ {
			Expect(rateLimiter.Acquire()).NotTo(BeTrue())
		}
		Expect(rateLimiter.Acquire()).To(BeTrue())
	})

	It("test rampup ratelimiter start and stop many times", func() {
		Expect(func() {
			ratelimiter, _ := NewRampUpRateLimiter(100, "10/200ms", 100*time.Millisecond)
			for i := 0; i < 500; i++ {
				ratelimiter.Start()
				time.Sleep(5 * time.Millisecond)
				ratelimiter.Stop()
			}
		}).Should(Not(Panic()))
	})

	It("test parse rampup rate", func() {
		rateLimiter := &RampUpRateLimiter{}
		rampUpStep, rampUpPeriod, _ := rateLimiter.parseRampUpRate("100")
		Expect(rampUpStep).To(BeEquivalentTo(100))
		Expect(rampUpPeriod).To(BeEquivalentTo(time.Second))

		rampUpStep, rampUpPeriod, _ = rateLimiter.parseRampUpRate("200/10s")
		Expect(rampUpStep).To(BeEquivalentTo(200))
		Expect(rampUpPeriod).To(BeEquivalentTo(10 * time.Second))
	})

	DescribeTable("test parse invalid rampup rate", func(rampupRate string) {
		rateLimiter := &RampUpRateLimiter{}
		_, _, err := rateLimiter.parseRampUpRate(rampupRate)
		Expect(err).Should(HaveOccurred())
		Expect(err).Should(BeIdenticalTo(ErrParsingRampUpRate))
	},
		Entry("Invalid", "A/1m"),
		Entry("Invalid", "A"),
		Entry("Invalid", "200/1s/"),
		Entry("Invalid", "200/1"),
		Entry("Invalid", "200/1"),
	)

})

// Stop used to panic with "close of nil channel" when it was called before Start.
func TestStableRateLimiterStopWithoutStart(t *testing.T) {
	limiter := NewStableRateLimiter(10, time.Second)
	limiter.Stop()
	// Stop must be idempotent.
	limiter.Stop()
}

func TestRampUpRateLimiterStopWithoutStart(t *testing.T) {
	limiter, err := NewRampUpRateLimiter(10, "1", time.Second)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}
	limiter.Stop()
	limiter.Stop()
}

// Acquire used to block forever when the limiter was not running.
func TestStableRateLimiterAcquireWithoutStart(t *testing.T) {
	limiter := NewStableRateLimiter(1, time.Second)
	if limiter.Acquire() {
		t.Fatal("the first Acquire should be allowed")
	}

	blocked := make(chan bool, 1)
	go func() {
		blocked <- limiter.Acquire()
	}()

	select {
	case result := <-blocked:
		if !result {
			t.Fatal("expected Acquire to report that it was blocked")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire is still blocked although the limiter is not running")
	}
}

// Stop must release the goroutines which are blocked in Acquire, otherwise the
// workers are stuck forever after the test is stopped.
func TestStableRateLimiterStopReleasesBlockedAcquire(t *testing.T) {
	// A long refill period guarantees that the bucket is not refilled during the test.
	limiter := NewStableRateLimiter(1, time.Hour)
	limiter.Start()
	defer limiter.Stop()

	if limiter.Acquire() {
		t.Fatal("the first Acquire should be allowed")
	}

	returned := make(chan bool, 1)
	go func() {
		returned <- limiter.Acquire()
	}()

	// Make sure the goroutine is really blocked in Acquire.
	time.Sleep(100 * time.Millisecond)
	select {
	case <-returned:
		t.Fatal("Acquire returned before Stop")
	default:
	}

	limiter.Stop()

	select {
	case result := <-returned:
		if !result {
			t.Fatal("expected Acquire to report that it was blocked")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire is still blocked after Stop")
	}
}

func TestRampUpRateLimiterStopReleasesBlockedAcquire(t *testing.T) {
	limiter, err := NewRampUpRateLimiter(1, "1", time.Hour)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}
	limiter.Start()
	defer limiter.Stop()

	if limiter.Acquire() {
		t.Fatal("the first Acquire should be allowed")
	}

	returned := make(chan bool, 1)
	go func() {
		returned <- limiter.Acquire()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case <-returned:
		t.Fatal("Acquire returned before Stop")
	default:
	}

	limiter.Stop()

	select {
	case result := <-returned:
		if !result {
			t.Fatal("expected Acquire to report that it was blocked")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire is still blocked after Stop")
	}
}

// A limiter stopped and started again must keep working, Stop used to leave the
// blocked goroutines behind with no way to wake them up.
func TestStableRateLimiterStartAfterStop(t *testing.T) {
	limiter := NewStableRateLimiter(1, 10*time.Millisecond)
	limiter.Start()
	limiter.Stop()
	limiter.Start()
	defer limiter.Stop()

	deadline := time.After(5 * time.Second)
	for {
		acquired := make(chan struct{}, 1)
		go func() {
			limiter.Acquire()
			acquired <- struct{}{}
		}()

		select {
		case <-acquired:
			return
		case <-deadline:
			t.Fatal("Acquire never returns after restarting the limiter")
		case <-time.After(100 * time.Millisecond):
			// Still blocked, the bucket should be refilled soon.
		}
	}
}
