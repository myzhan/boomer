package boomer

import (
	"flag"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/myzhan/gomq/zmtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test Boomer", func() {

	It("test new instance", func() {
		b := NewBoomer("0.0.0.0", 1234)
		Expect(b.masterHost).To(Equal("0.0.0.0"))
		Expect(b.masterPort).To(Equal(1234))
		Expect(b.mode).To(Equal(DistributedMode))
	})

	It("test new standalone instance", func() {
		b := NewStandaloneBoomer(100, 10)
		Expect(b.spawnCount).To(Equal(100))
		Expect(b.spawnRate).To(BeEquivalentTo(10))
		Expect(b.mode).To(Equal(StandaloneMode))
	})

	It("test set ratelimiter", func() {
		b := NewStandaloneBoomer(100, 10)
		limiter, _ := NewRampUpRateLimiter(10, "10/1s", time.Second)
		b.SetRateLimiter(limiter)
		Expect(b.rateLimiter).NotTo(BeNil())
	})

	It("test set mode", func() {
		b := NewStandaloneBoomer(100, 10)
		b.SetMode(DistributedMode)
		Expect(b.mode).To(Equal(DistributedMode))

		b.SetMode(StandaloneMode)
		Expect(b.mode).To(Equal(StandaloneMode))

		b.SetMode(3)
		Expect(b.mode).To(Equal(StandaloneMode))
	})

	It("test add output", func() {
		b := NewStandaloneBoomer(100, 10)
		b.AddOutput(NewConsoleOutput())
		b.AddOutput(NewConsoleOutput())
		Expect(b.outputs).To(HaveLen(2))
	})

	It("test enable CPU profile", func() {
		b := NewStandaloneBoomer(100, 10)
		b.EnableCPUProfile("cpu.prof", time.Second)
		Expect(b.cpuProfileFile).To(Equal("cpu.prof"))
		Expect(b.cpuProfileDuration).To(Equal(time.Second))
	})

	It("test enable memory profile", func() {
		b := NewStandaloneBoomer(100, 10)
		b.EnableMemoryProfile("mem.prof", time.Second)
		Expect(b.memoryProfileFile).To(Equal("mem.prof"))
		Expect(b.memoryProfileDuration).To(Equal(time.Second))
	})

	It("test standalone run", func() {
		b := NewStandaloneBoomer(10, 10)
		b.EnableCPUProfile("cpu.pprof", 2*time.Second)
		b.EnableMemoryProfile("mem.pprof", 2*time.Second)

		count := int64(0)
		taskA := &Task{
			Name: "increaseCount",
			Fn: func() {
				atomic.AddInt64(&count, 1)
				runtime.Goexit()
			},
		}

		go b.Run(taskA)
		defer b.Quit()
		defer os.Remove("cpu.pprof")
		defer os.Remove("mem.pprof")

		Eventually(func() int64 { return atomic.LoadInt64(&count) }).Should(BeEquivalentTo(10))
		Eventually(func() string { return "cpu.pprof" }).Should(BeAnExistingFile())
		Eventually(func() string { return "mem.pprof" }).Should(BeAnExistingFile())
	})

	It("test distributed run", func() {
		masterHost := "mock:0.0.0.0"
		masterPort := 10240

		b := NewBoomer(masterHost, masterPort)

		count := int64(0)
		taskA := &Task{
			Name: "increaseCount",
			Fn: func() {
				atomic.AddInt64(&count, 1)
				runtime.Goexit()
			},
		}
		b.Run(taskA)
		defer b.Quit()

		serverMessage := newGenericMessage("spawn", map[string]interface{}{
			"user_classes_count": map[interface{}]interface{}{
				"Dummy":  int64(5),
				"Dummy2": int64(5),
			},
		}, b.slaveRunner.nodeID)
		serverMessageInBytes, _ := serverMessage.serialize()
		serverZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{serverMessageInBytes},
		}
		mockGomqDealerInstance.RecvChannel() <- serverZmtpMessage

		time.Sleep(4 * time.Second)
		Expect(atomic.LoadInt64(&count)).Should(BeEquivalentTo(10))
	})

	It("test run tasks for test", func() {
		defer func() {
			runTasks = ""
		}()

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

		Expect(count).To(Equal(1))
	})

	It("test create ratelimiter", func() {
		rateLimiter, err := createRateLimiter(100, "-1")
		Expect(rateLimiter).To(BeAssignableToTypeOf(&StableRateLimiter{}))
		Expect(err).ShouldNot(HaveOccurred())

		stableRateLimiter, _ := rateLimiter.(*StableRateLimiter)
		Expect(stableRateLimiter.threshold).To(BeEquivalentTo(100))

		rateLimiter, err = createRateLimiter(0, "1")
		Expect(rateLimiter).To(BeAssignableToTypeOf(&RampUpRateLimiter{}))
		Expect(err).ShouldNot(HaveOccurred())

		rampUpRateLimiter, _ := rateLimiter.(*RampUpRateLimiter)
		Expect(rampUpRateLimiter.maxThreshold).To(BeEquivalentTo(math.MaxInt64))
		Expect(rampUpRateLimiter.rampUpRate).To(Equal("1"))

		rateLimiter, err = createRateLimiter(10, "2/2s")
		Expect(rateLimiter).To(BeAssignableToTypeOf(&RampUpRateLimiter{}))
		Expect(err).ShouldNot(HaveOccurred())

		rampUpRateLimiter, _ = rateLimiter.(*RampUpRateLimiter)
		Expect(rampUpRateLimiter.maxThreshold).To(BeEquivalentTo(10))
		Expect(rampUpRateLimiter.rampUpRate).To(Equal("2/2s"))
		Expect(rampUpRateLimiter.rampUpStep).To(BeEquivalentTo(2))
		Expect(rampUpRateLimiter.rampUpPeriod).To(BeEquivalentTo(2 * time.Second))
	})

	It("test run", func() {
		flag.Parse()
		masterHost = "mock:0.0.0.0"
		masterPort = 1234
		count := int64(0)
		taskA := &Task{
			Name: "increaseCount",
			Fn: func() {
				atomic.AddInt64(&count, 1)
				runtime.Goexit()
			},
		}

		go Run(taskA)
		defer defaultBoomer.Quit()

		// Wait for Run() to finish setting up slaveRunner
		time.Sleep(200 * time.Millisecond)

		defaultBoomer.mu.RLock()
		sr := defaultBoomer.slaveRunner
		defaultBoomer.mu.RUnlock()
		Expect(sr).NotTo(BeNil())

		// Drain the client_ready message from the send channel
		Eventually(mockGomqDealerInstance.SendChannel()).Should(Receive())

		serverMessage := newGenericMessage("spawn", map[string]interface{}{
			"user_classes_count": map[interface{}]interface{}{
				"Dummy":  int64(5),
				"Dummy2": int64(5),
			},
		}, sr.nodeID)
		serverMessageInBytes, _ := serverMessage.serialize()
		serverZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{serverMessageInBytes},
		}
		mockGomqDealerInstance.RecvChannel() <- serverZmtpMessage

		time.Sleep(4 * time.Second)
		Expect(atomic.LoadInt64(&count)).To(BeEquivalentTo(10))
	})

	It("test record success", func() {
		defer func() {
			defaultBoomer = &Boomer{logger: log.Default()}
		}()

		// called before runner instance created
		RecordSuccess("http", "foo", int64(1), int64(10))

		// distribute mode
		masterHost := "127.0.0.1"
		masterPort := 5557
		defaultBoomer = NewBoomer(masterHost, masterPort)
		defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)
		RecordSuccess("http", "foo", int64(1), int64(10))

		var requestSuccessMsg *requestSuccess
		Expect(defaultBoomer.slaveRunner.stats.requestSuccessChan).Should(Receive(&requestSuccessMsg))
		Expect(requestSuccessMsg.requestType).To(Equal("http"))
		Expect(requestSuccessMsg.responseTime).To(BeEquivalentTo(1))

		// standalone mode
		defaultBoomer = NewStandaloneBoomer(1, 1)
		defaultBoomer.localRunner = newLocalRunner(nil, nil, 1, 1)
		RecordSuccess("http", "foo", int64(1), int64(10))

		Expect(defaultBoomer.localRunner.stats.requestSuccessChan).Should(Receive(&requestSuccessMsg))
		Expect(requestSuccessMsg.requestType).To(Equal("http"))
		Expect(requestSuccessMsg.responseTime).To(BeEquivalentTo(1))
	})

	It("test record failure", func() {
		defer func() {
			defaultBoomer = &Boomer{logger: log.Default()}
		}()

		// called before runner instance created
		RecordFailure("udp", "bar", int64(2), "udp error")

		// distribute mode
		masterHost := "127.0.0.1"
		masterPort := 5557
		defaultBoomer = NewBoomer(masterHost, masterPort)
		defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)
		RecordFailure("udp", "bar", int64(2), "udp error")

		var requestFailureMsg *requestFailure
		Expect(defaultBoomer.slaveRunner.stats.requestFailureChan).To(Receive(&requestFailureMsg))
		Expect(requestFailureMsg.requestType).To(Equal("udp"))
		Expect(requestFailureMsg.responseTime).To(BeEquivalentTo(2))
		Expect(requestFailureMsg.error).To(Equal("udp error"))

		// standalone mode
		defaultBoomer = NewStandaloneBoomer(1, 1)
		defaultBoomer.localRunner = newLocalRunner(nil, nil, 1, 1)
		RecordFailure("udp", "bar", int64(2), "udp error")

		Expect(defaultBoomer.localRunner.stats.requestFailureChan).To(Receive(&requestFailureMsg))
		Expect(requestFailureMsg.requestType).To(Equal("udp"))
		Expect(requestFailureMsg.responseTime).To(BeEquivalentTo(2))
		Expect(requestFailureMsg.error).To(Equal("udp error"))
	})

	It("test loggers", func() {
		defer func() {
			defaultBoomer = &Boomer{logger: log.Default()}
		}()

		logger := log.New(os.Stdout, "[boomer]", log.LstdFlags)

		defaultBoomer = &Boomer{}
		defaultBoomer.WithLogger(nil)
		defaultBoomer.WithLogger(logger)

		defaultBoomer.slaveRunner = &slaveRunner{}
		defaultBoomer.localRunner = &localRunner{}
		defaultBoomer.WithLogger(logger)
	})
})

// RecordSuccess used to dereference the runner of the current mode even when only
// the other runner existed, which panicked with a nil pointer dereference.
func TestRecordWithMismatchedRunner(t *testing.T) {
	b := NewBoomer("127.0.0.1", 5557) // DistributedMode
	b.localRunner = newLocalRunner([]*Task{noopTask("noop")}, nil, 1, 1)

	b.RecordSuccess("http", "foo", 1, 1)
	b.RecordFailure("http", "foo", 1, "boom")
	b.SendCustomMessage("custom", nil)

	standalone := NewStandaloneBoomer(1, 1) // StandaloneMode
	standalone.slaveRunner = newSlaveRunner("127.0.0.1", 5557, nil, nil)

	standalone.RecordSuccess("http", "foo", 1, 1)
	standalone.RecordFailure("http", "foo", 1, "boom")
}

// Records must be dropped instead of blocking forever once stats is closed.
func TestRecordAfterStatsClosedDoesNotBlock(t *testing.T) {
	b := NewStandaloneBoomer(1, 1)
	lr := newLocalRunner([]*Task{noopTask("noop")}, nil, 1, 1)
	lr.setLogger(newDiscardLogger())
	b.localRunner = lr
	lr.stats.close()

	done := make(chan struct{})
	go func() {
		// Fill the buffered channels so that sending can only give up on the
		// closed stats.
		for i := 0; i < cap(lr.stats.requestSuccessChan)+1; i++ {
			b.RecordSuccess("http", "foo", 1, 1)
			b.RecordFailure("http", "foo", 1, "boom")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RecordSuccess/RecordFailure blocked after stats was closed")
	}
}

// The hot record path must not take the lock on every call, Run caches the stats
// pointer of the runner it starts.
func TestRunCachesRecordStats(t *testing.T) {
	b := NewStandaloneBoomer(1, 1).WithLogger(newDiscardLogger())
	go b.Run(noopTask("noop"))
	defer b.Quit()

	var stats *requestStats
	for i := 0; i < 200; i++ {
		if stats = b.recordStats.Load(); stats != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if stats == nil {
		t.Fatal("run did not cache the stats of the local runner")
	}

	b.mu.RLock()
	lr := b.localRunner
	b.mu.RUnlock()
	if lr == nil || stats != lr.stats {
		t.Fatal("cached stats does not belong to the running local runner")
	}
}

func TestQuitIsIdempotent(t *testing.T) {
	b := NewStandaloneBoomer(1, 1)
	lr := newLocalRunner([]*Task{noopTask("noop")}, nil, 1, 1)
	lr.setLogger(newDiscardLogger())
	b.localRunner = lr

	b.Quit()
	b.Quit()

	select {
	case <-lr.shutdownChan:
	default:
		t.Fatal("local runner was not shut down by Quit")
	}
}

func TestConcurrentRecordAfterStatsClosed(t *testing.T) {
	for _, failure := range []bool{false, true} {
		for attempt := 0; attempt < 100; attempt++ {
			b := NewStandaloneBoomer(1, 1)
			s := newRequestStats()
			s.close()
			b.recordStats.Store(s)
			// Leave one slot so concurrent senders compete for the remaining space.
			if failure {
				for i := 0; i < cap(s.requestFailureChan)-1; i++ {
					s.requestFailureChan <- &requestFailure{}
				}
			} else {
				for i := 0; i < cap(s.requestSuccessChan)-1; i++ {
					s.requestSuccessChan <- &requestSuccess{}
				}
			}
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					if failure {
						b.RecordFailure("GET", "closed", 1, "error")
					} else {
						b.RecordSuccess("GET", "closed", 1, 1)
					}
				}()
			}
			close(start)
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("concurrent records blocked after close (failure=%v)", failure)
			}
		}
	}
}

func TestQuitWithoutRunner(t *testing.T) {
	b := NewBoomer("127.0.0.1", 5557)
	b.Quit()

	standalone := NewStandaloneBoomer(1, 1)
	standalone.Quit()
}

// Run without tasks used to panic with "integer divide by zero" once workers
// were spawned.
func TestRunWithoutTasks(t *testing.T) {
	b := NewStandaloneBoomer(1, 1).WithLogger(newDiscardLogger())

	done := make(chan struct{})
	go func() {
		b.Run()
		b.Run(nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run without tasks blocked")
	}
}
