package boomer

import (
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
