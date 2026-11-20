package boomer

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter is used to put limits on task executions.
type RateLimiter interface {
	// Start is used to enable the rate limiter.
	// It can be implemented as a noop if not needed.
	Start()

	// Acquire() is called before executing a task.Fn function.
	// If Acquire() returns true, the task.Fn function will be executed.
	// If Acquire() returns false, the task.Fn function won't be executed this time, but Acquire() will be called very soon.
	// It works like:
	// for {
	//      blocked := rateLimiter.Acquire()
	//      if !blocked {
	//	        task.Fn()
	//      }
	// }
	// Acquire() should block the caller until execution is allowed.
	Acquire() bool

	// Stop is used to disable the rate limiter.
	// It can be implemented as a noop if not needed.
	Stop()
}

// A StableRateLimiter uses the token bucket algorithm.
// the bucket is refilled according to the refill period, no burst is allowed.
type StableRateLimiter struct {
	threshold        int64
	currentThreshold int64
	timerId          uint64
	refillPeriod     time.Duration
	broadcastChannel chan bool
	mu               sync.Mutex
	quitChannel      chan bool
	// doneChannel is closed by Stop so that goroutines blocked in Acquire can return
	// instead of waiting for a refill which will never happen. It is nil before Start
	// and after Stop.
	doneChannel chan struct{}
}

// NewStableRateLimiter returns a StableRateLimiter.
func NewStableRateLimiter(threshold int64, refillPeriod time.Duration) (rateLimiter *StableRateLimiter) {
	rateLimiter = &StableRateLimiter{
		threshold:        threshold,
		currentThreshold: threshold,
		timerId:          uint64(0),
		refillPeriod:     refillPeriod,
		broadcastChannel: make(chan bool),
	}
	return rateLimiter
}

// Start to refill the bucket periodically.
func (limiter *StableRateLimiter) Start() {
	limiter.mu.Lock()
	limiter.quitChannel = make(chan bool)
	limiter.doneChannel = make(chan struct{})
	quitChannel := limiter.quitChannel
	limiter.mu.Unlock()

	timerId := atomic.AddUint64(&limiter.timerId, 1)
	go func(myId uint64) {
		for {
			select {
			case <-quitChannel:
				return
			default:
			}

			time.Sleep(limiter.refillPeriod)

			// Stop may have been called while sleeping, don't refill anymore.
			select {
			case <-quitChannel:
				return
			default:
			}

			hasBeenStarted := atomic.LoadUint64(&limiter.timerId) != myId
			if hasBeenStarted {
				return
			}

			atomic.StoreInt64(&limiter.currentThreshold, limiter.threshold)
			limiter.mu.Lock()
			close(limiter.broadcastChannel)
			limiter.broadcastChannel = make(chan bool)
			limiter.mu.Unlock()
		}
	}(timerId)
}

// Acquire a token from the bucket, returns true if the bucket is exhausted.
func (limiter *StableRateLimiter) Acquire() (blocked bool) {
	permit := atomic.AddInt64(&limiter.currentThreshold, -1)
	if permit >= 0 {
		return false
	}

	limiter.mu.Lock()
	broadcastChannel := limiter.broadcastChannel
	doneChannel := limiter.doneChannel
	limiter.mu.Unlock()

	if doneChannel == nil {
		// The limiter is not running, never block forever.
		return true
	}

	select {
	case <-broadcastChannel:
	case <-doneChannel:
	}
	return true
}

// Stop the rate limiter. It is safe to call Stop without Start, and to call it many times.
func (limiter *StableRateLimiter) Stop() {
	limiter.mu.Lock()
	quitChannel := limiter.quitChannel
	doneChannel := limiter.doneChannel
	limiter.quitChannel = nil
	limiter.doneChannel = nil
	if quitChannel != nil || doneChannel != nil {
		// Release goroutines blocked in Acquire.
		close(limiter.broadcastChannel)
		limiter.broadcastChannel = make(chan bool)
	}
	limiter.mu.Unlock()

	if quitChannel != nil {
		close(quitChannel)
	}
	if doneChannel != nil {
		close(doneChannel)
	}
}

// ErrParsingRampUpRate is the error returned if the format of rampUpRate is invalid.
var ErrParsingRampUpRate = errors.New("ratelimiter: invalid format of rampUpRate, try \"1\" or \"1/1s\"")

// A RampUpRateLimiter uses the token bucket algorithm.
// the threshold is updated according to the warm up rate.
// the bucket is refilled according to the refill period, no burst is allowed.
type RampUpRateLimiter struct {
	maxThreshold     int64
	nextThreshold    int64
	currentThreshold int64
	timerId          uint64
	refillPeriod     time.Duration
	rampUpRate       string
	rampUpStep       int64
	rampUpPeriod     time.Duration
	broadcastChannel chan bool
	mu               sync.Mutex
	quitChannel      chan bool
	// doneChannel is closed by Stop so that goroutines blocked in Acquire can return.
	// It is nil before Start and after Stop.
	doneChannel chan struct{}
}

// NewRampUpRateLimiter returns a RampUpRateLimiter.
// Valid formats of rampUpRate are "1", "1/1s".
func NewRampUpRateLimiter(maxThreshold int64, rampUpRate string, refillPeriod time.Duration) (rateLimiter *RampUpRateLimiter, err error) {
	rateLimiter = &RampUpRateLimiter{
		maxThreshold:     maxThreshold,
		nextThreshold:    0,
		currentThreshold: 0,
		timerId:          uint64(0),
		rampUpRate:       rampUpRate,
		refillPeriod:     refillPeriod,
		broadcastChannel: make(chan bool),
	}
	rateLimiter.rampUpStep, rateLimiter.rampUpPeriod, err = rateLimiter.parseRampUpRate(rateLimiter.rampUpRate)
	if err != nil {
		return nil, err
	}
	return rateLimiter, nil
}

func (limiter *RampUpRateLimiter) parseRampUpRate(rampUpRate string) (rampUpStep int64, rampUpPeriod time.Duration, err error) {
	if strings.Contains(rampUpRate, "/") {
		tmp := strings.Split(rampUpRate, "/")
		if len(tmp) != 2 {
			return rampUpStep, rampUpPeriod, ErrParsingRampUpRate
		}
		rampUpStep, err := strconv.ParseInt(tmp[0], 10, 64)
		if err != nil {
			return rampUpStep, rampUpPeriod, ErrParsingRampUpRate
		}
		rampUpPeriod, err := time.ParseDuration(tmp[1])
		if err != nil {
			return rampUpStep, rampUpPeriod, ErrParsingRampUpRate
		}
		return rampUpStep, rampUpPeriod, nil
	}

	rampUpStep, err = strconv.ParseInt(rampUpRate, 10, 64)
	if err != nil {
		return rampUpStep, rampUpPeriod, ErrParsingRampUpRate
	}
	rampUpPeriod = time.Second
	return rampUpStep, rampUpPeriod, nil
}

func (limiter *RampUpRateLimiter) getNextThreshold() int64 {
	nextValue := atomic.LoadInt64(&limiter.nextThreshold) + limiter.rampUpStep
	if nextValue < 0 {
		// int64 overflow
		nextValue = int64(math.MaxInt64)
	}
	if nextValue > limiter.maxThreshold {
		nextValue = limiter.maxThreshold
	}
	return nextValue
}

// Start to refill the bucket periodically.
func (limiter *RampUpRateLimiter) Start() {
	limiter.mu.Lock()
	limiter.quitChannel = make(chan bool)
	limiter.doneChannel = make(chan struct{})
	quitChannel := limiter.quitChannel
	limiter.mu.Unlock()

	timerId := atomic.AddUint64(&limiter.timerId, 1)

	atomic.StoreInt64(&limiter.nextThreshold, limiter.getNextThreshold())
	atomic.StoreInt64(&limiter.currentThreshold, atomic.LoadInt64(&limiter.nextThreshold))
	// bucket updater
	go func(myId uint64) {
		for {
			select {
			case <-quitChannel:
				return
			default:
			}

			time.Sleep(limiter.refillPeriod)

			// Stop may have been called while sleeping, don't refill anymore.
			select {
			case <-quitChannel:
				return
			default:
			}

			hasBeenStarted := atomic.LoadUint64(&limiter.timerId) != myId
			if hasBeenStarted {
				return
			}

			atomic.StoreInt64(&limiter.currentThreshold, atomic.LoadInt64(&limiter.nextThreshold))
			limiter.mu.Lock()
			close(limiter.broadcastChannel)
			limiter.broadcastChannel = make(chan bool)
			limiter.mu.Unlock()
		}
	}(timerId)

	// threshold updater
	go func(myId uint64) {
		for {
			select {
			case <-quitChannel:
				return
			default:
			}

			time.Sleep(limiter.rampUpPeriod)

			// Stop may have been called while sleeping, don't ramp up anymore.
			select {
			case <-quitChannel:
				return
			default:
			}

			hasBeenStarted := atomic.LoadUint64(&limiter.timerId) != myId
			if hasBeenStarted {
				return
			}

			atomic.StoreInt64(&limiter.nextThreshold, limiter.getNextThreshold())
		}
	}(timerId)
}

// Acquire a token from the bucket, returns true if the bucket is exhausted.
func (limiter *RampUpRateLimiter) Acquire() (blocked bool) {
	permit := atomic.AddInt64(&limiter.currentThreshold, -1)
	if permit >= 0 {
		return false
	}

	limiter.mu.Lock()
	broadcastChannel := limiter.broadcastChannel
	doneChannel := limiter.doneChannel
	limiter.mu.Unlock()

	if doneChannel == nil {
		// The limiter is not running, never block forever.
		return true
	}

	select {
	case <-broadcastChannel:
	case <-doneChannel:
	}
	return true
}

// Stop the rate limiter. It is safe to call Stop without Start, and to call it many times.
func (limiter *RampUpRateLimiter) Stop() {
	atomic.StoreInt64(&limiter.nextThreshold, 0)

	limiter.mu.Lock()
	quitChannel := limiter.quitChannel
	doneChannel := limiter.doneChannel
	limiter.quitChannel = nil
	limiter.doneChannel = nil
	if quitChannel != nil || doneChannel != nil {
		// Release goroutines blocked in Acquire.
		close(limiter.broadcastChannel)
		limiter.broadcastChannel = make(chan bool)
	}
	limiter.mu.Unlock()

	if quitChannel != nil {
		close(quitChannel)
	}
	if doneChannel != nil {
		close(doneChannel)
	}
}
