package boomer

import (
	"flag"
	"log"
	"math"
	"os"
	"runtime"
	"sync/atomic"
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
		MockGomqDealerInstance.RecvChannel() <- serverZmtpMessage

		time.Sleep(4 * time.Second)
		Expect(count).Should(BeEquivalentTo(10))
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
		Expect(rampUpRateLimiter.rampUpPeroid).To(BeEquivalentTo(2 * time.Second))
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
		time.Sleep(50 * time.Millisecond)
		defer defaultBoomer.Quit()

		serverMessage := newGenericMessage("spawn", map[string]interface{}{
			"user_classes_count": map[interface{}]interface{}{
				"Dummy":  int64(5),
				"Dummy2": int64(5),
			},
		}, defaultBoomer.slaveRunner.nodeID)
		serverMessageInBytes, _ := serverMessage.serialize()
		serverZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{serverMessageInBytes},
		}
		MockGomqDealerInstance.RecvChannel() <- serverZmtpMessage

		time.Sleep(4 * time.Second)
		Expect(count).To(BeEquivalentTo(10))
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
