package boomer

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// runner uses a rate limiter to put limits on task executions.
type rateLimiter interface {
	start()
	acquire() bool
	stop()
}

// stableRateLimiter uses the token bucket algorithm.
// the bucket is refilled according to the refill period, no burst is allowed.
type stableRateLimiter struct {
	threshold        int64
	currentThreshold int64
	refillPeroid     time.Duration
	broadcastChannel chan bool
	quitChannel      chan bool
}

func newStableRateLimiter(threshold int64, refillPeroid time.Duration) (rateLimiter *stableRateLimiter) {
	rateLimiter = &stableRateLimiter{
		threshold:        threshold,
		currentThreshold: threshold,
		refillPeroid:     refillPeroid,
		broadcastChannel: make(chan bool),
	}
	return rateLimiter
}

func (limiter *stableRateLimiter) start() {
	limiter.quitChannel = make(chan bool)
	quitChannel := limiter.quitChannel
	go func() {
		for {
			select {
			case <-quitChannel:
				return
			default:
				atomic.StoreInt64(&limiter.currentThreshold, limiter.threshold)
				time.Sleep(limiter.refillPeroid)
				close(limiter.broadcastChannel)
				limiter.broadcastChannel = make(chan bool)
			}
		}
	}()
}

func (limiter *stableRateLimiter) acquire() (blocked bool) {
	permit := atomic.AddInt64(&limiter.currentThreshold, -1)
	if permit < 0 {
		blocked = true
		// block until the bucket is refilled
		<-limiter.broadcastChannel
	} else {
		blocked = false
	}
	return blocked
}

func (limiter *stableRateLimiter) stop() {
	close(limiter.quitChannel)
}

// ErrParsingRampUpRate is the error returned if the format of rampUpRate is invalid.
var ErrParsingRampUpRate = errors.New("ratelimiter: invalid format of rampUpRate, try \"1\" or \"1/1s\"")

// rampUpRateLimiter uses the token bucket algorithm.
// the threshold is updated according to the warm up rate.
// the bucket is refilled according to the refill period, no burst is allowed.
type rampUpRateLimiter struct {
	maxThreshold     int64
	nextThreshold    int64
	currentThreshold int64
	refillPeroid     time.Duration
	rampUpRate       string
	rampUpStep       int64
	rampUpPeroid     time.Duration
	broadcastChannel chan bool
	rampUpChannel    chan bool
	quitChannel      chan bool
}

func newRampUpRateLimiter(maxThreshold int64, rampUpRate string, refillPeroid time.Duration) (rateLimiter *rampUpRateLimiter, err error) {
	rateLimiter = &rampUpRateLimiter{
		maxThreshold:     maxThreshold,
		nextThreshold:    0,
		currentThreshold: 0,
		rampUpRate:       rampUpRate,
		refillPeroid:     refillPeroid,
		broadcastChannel: make(chan bool),
	}
	rateLimiter.rampUpStep, rateLimiter.rampUpPeroid, err = rateLimiter.parseRampUpRate(rateLimiter.rampUpRate)
	if err != nil {
		return nil, err
	}
	return rateLimiter, nil
}

func (limiter *rampUpRateLimiter) parseRampUpRate(rampUpRate string) (rampUpStep int64, rampUpPeroid time.Duration, err error) {
	if strings.Contains(rampUpRate, "/") {
		tmp := strings.Split(rampUpRate, "/")
		if len(tmp) != 2 {
			return rampUpStep, rampUpPeroid, ErrParsingRampUpRate
		}
		rampUpStep, err := strconv.ParseInt(tmp[0], 10, 64)
		if err != nil {
			return rampUpStep, rampUpPeroid, ErrParsingRampUpRate
		}
		rampUpPeroid, err := time.ParseDuration(tmp[1])
		if err != nil {
			return rampUpStep, rampUpPeroid, ErrParsingRampUpRate
		}
		return rampUpStep, rampUpPeroid, nil
	}

	rampUpStep, err = strconv.ParseInt(rampUpRate, 10, 64)
	if err != nil {
		return rampUpStep, rampUpPeroid, ErrParsingRampUpRate
	}
	rampUpPeroid = time.Second
	return rampUpStep, rampUpPeroid, nil
}

func (limiter *rampUpRateLimiter) start() {
	limiter.quitChannel = make(chan bool)
	quitChannel := limiter.quitChannel
	// bucket updater
	go func() {
		for {
			select {
			case <-quitChannel:
				return
			default:
				atomic.StoreInt64(&limiter.currentThreshold, limiter.nextThreshold)
				time.Sleep(limiter.refillPeroid)
				close(limiter.broadcastChannel)
				limiter.broadcastChannel = make(chan bool)
			}
		}
	}()
	// threshold updater
	go func() {
		for {
			select {
			case <-quitChannel:
				return
			default:
				limiter.nextThreshold = limiter.nextThreshold + limiter.rampUpStep
				if limiter.nextThreshold < 0 {
					// int64 overflow
					limiter.nextThreshold = int64(math.MaxInt64)
				}
				if limiter.nextThreshold > limiter.maxThreshold {
					limiter.nextThreshold = limiter.maxThreshold
				}
				time.Sleep(limiter.rampUpPeroid)
			}
		}
	}()
}

func (limiter *rampUpRateLimiter) acquire() (blocked bool) {
	permit := atomic.AddInt64(&limiter.currentThreshold, -1)
	if permit < 0 {
		blocked = true
		// block until the bucket is refilled
		<-limiter.broadcastChannel
	} else {
		blocked = false
	}
	return blocked
}

func (limiter *rampUpRateLimiter) stop() {
	limiter.nextThreshold = 0
	close(limiter.quitChannel)
}
