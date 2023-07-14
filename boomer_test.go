package boomer

import (
	"flag"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBoomer(t *testing.T) {
	b := NewBoomer("0.0.0.0", 1234)
	assert.Equal(t, "0.0.0.0", b.masterHost)
	assert.Equal(t, 1234, b.masterPort)
	assert.Equal(t, DistributedMode, b.mode)
}

func TestNewStandaloneBoomer(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	assert.Equal(t, 100, b.spawnCount)
	assert.Equal(t, float64(10), b.spawnRate)
	assert.Equal(t, StandaloneMode, b.mode)
}

func TestSetRateLimiter(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	limiter, _ := NewRampUpRateLimiter(10, "10/1s", time.Second)
	b.SetRateLimiter(limiter)
	assert.NotNil(t, b.rateLimiter)
}

func TestSetMode(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)

	b.SetMode(DistributedMode)
	assert.Equal(t, DistributedMode, b.mode)

	b.SetMode(StandaloneMode)
	assert.Equal(t, StandaloneMode, b.mode)

	b.SetMode(3)
	assert.Equal(t, StandaloneMode, b.mode)
}

func TestAddOutput(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	b.AddOutput(NewConsoleOutput())
	b.AddOutput(NewConsoleOutput())

	assert.Len(t, b.outputs, 2)
}

func TestEnableCPUProfile(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	b.EnableCPUProfile("cpu.prof", time.Second)
	assert.Equal(t, "cpu.prof", b.cpuProfileFile)
	assert.Equal(t, time.Second, b.cpuProfileDuration)
}

func TestEnableMemoryProfile(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	b.EnableMemoryProfile("mem.prof", time.Second)
	assert.Equal(t, "mem.prof", b.memoryProfileFile)
	assert.Equal(t, time.Second, b.memoryProfileDuration)
}

func TestStandaloneRun(t *testing.T) {
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

	time.Sleep(5 * time.Second)

	b.Quit()

	assert.Equal(t, int64(10), count)
	assert.FileExists(t, "cpu.pprof")
	_ = os.Remove("cpu.pprof")

	assert.FileExists(t, "mem.pprof")
	_ = os.Remove("mem.pprof")
}

func TestDistributedRun(t *testing.T) {
	masterHost := "0.0.0.0"
	rand.Seed(Now())
	masterPort := rand.Intn(1000) + 10240

	server := newTestServer(masterHost, masterPort)
	defer server.close()

	log.Printf("Starting to serve on %s:%d\n", masterHost, masterPort)
	server.start()

	time.Sleep(20 * time.Millisecond)

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

	server.toClient <- newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(5),
			"Dummy2": int64(5),
		},
	}, b.slaveRunner.nodeID)

	time.Sleep(4 * time.Second)

	b.Quit()

	assert.Equal(t, int64(10), count)
}

func TestRunTasksForTest(t *testing.T) {
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

	assert.Equal(t, 1, count)
	runTasks = ""
}

func TestCreateRatelimiter(t *testing.T) {
	rateLimiter, _ := createRateLimiter(100, "-1")
	assert.IsType(t, &StableRateLimiter{}, rateLimiter)
	stableRateLimiter, _ := rateLimiter.(*StableRateLimiter)
	assert.EqualValues(t, 100, stableRateLimiter.threshold)

	rateLimiter, _ = createRateLimiter(0, "1")
	assert.IsType(t, &RampUpRateLimiter{}, rateLimiter)
	rampUpRateLimiter, _ := rateLimiter.(*RampUpRateLimiter)
	assert.EqualValues(t, math.MaxInt64, rampUpRateLimiter.maxThreshold)
	assert.Equal(t, math.MaxInt64, math.MaxInt64)
	assert.Equal(t, "1", rampUpRateLimiter.rampUpRate)

	rateLimiter, _ = createRateLimiter(10, "2/2s")
	assert.IsType(t, &RampUpRateLimiter{}, rateLimiter)
	rampUpRateLimiter, _ = rateLimiter.(*RampUpRateLimiter)
	assert.EqualValues(t, 10, rampUpRateLimiter.maxThreshold)
	assert.Equal(t, "2/2s", rampUpRateLimiter.rampUpRate)
	assert.EqualValues(t, 2, rampUpRateLimiter.rampUpStep)
	assert.Equal(t, 2*time.Second, rampUpRateLimiter.rampUpPeroid)
}

func TestRun(t *testing.T) {
	flag.Parse()

	masterHost = "0.0.0.0"
	rand.Seed(Now())
	masterPort = rand.Intn(1000) + 10240

	server := newTestServer(masterHost, masterPort)
	defer server.close()

	log.Printf("Starting to serve on %s:%d\n", masterHost, masterPort)
	server.start()

	time.Sleep(20 * time.Millisecond)

	count := int64(0)
	taskA := &Task{
		Name: "increaseCount",
		Fn: func() {
			atomic.AddInt64(&count, 1)
			runtime.Goexit()
		},
	}

	go Run(taskA)
	time.Sleep(20 * time.Millisecond)

	server.toClient <- newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(5),
			"Dummy2": int64(5),
		},
	}, defaultBoomer.slaveRunner.nodeID)

	time.Sleep(4 * time.Second)

	defaultBoomer.Quit()

	assert.Equal(t, int64(10), count)
}

func TestRecordSuccess(t *testing.T) {
	// called before runner instance created
	RecordSuccess("http", "foo", int64(1), int64(10))

	// distribute mode
	masterHost := "127.0.0.1"
	masterPort := 5557
	defaultBoomer = NewBoomer(masterHost, masterPort)
	defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)
	RecordSuccess("http", "foo", int64(1), int64(10))

	requestSuccessMsg := <-defaultBoomer.slaveRunner.stats.requestSuccessChan
	assert.Equal(t, "http", requestSuccessMsg.requestType)
	assert.Equal(t, int64(1), requestSuccessMsg.responseTime)

	// standalone mode
	defaultBoomer = NewStandaloneBoomer(1, 1)
	defaultBoomer.localRunner = newLocalRunner(nil, nil, 1, 1)
	RecordSuccess("http", "foo", int64(1), int64(10))

	requestSuccessMsg = <-defaultBoomer.localRunner.stats.requestSuccessChan
	assert.Equal(t, "http", requestSuccessMsg.requestType)
	assert.Equal(t, int64(1), requestSuccessMsg.responseTime)

	defaultBoomer = &Boomer{}
}

func TestRecordFailure(t *testing.T) {
	// called before runner instance created
	RecordFailure("udp", "bar", int64(2), "udp error")

	// distribute mode
	masterHost := "127.0.0.1"
	masterPort := 5557
	defaultBoomer = NewBoomer(masterHost, masterPort)
	defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)
	RecordFailure("udp", "bar", int64(2), "udp error")

	requestFailureMsg := <-defaultBoomer.slaveRunner.stats.requestFailureChan
	assert.Equal(t, "udp", requestFailureMsg.requestType)
	assert.Equal(t, int64(2), requestFailureMsg.responseTime)
	assert.Equal(t, "udp error", requestFailureMsg.error)

	// standalone mode
	defaultBoomer = NewStandaloneBoomer(1, 1)
	defaultBoomer.localRunner = newLocalRunner(nil, nil, 1, 1)
	RecordFailure("udp", "bar", int64(2), "udp error")

	requestFailureMsg = <-defaultBoomer.localRunner.stats.requestFailureChan
	assert.Equal(t, "udp", requestFailureMsg.requestType)
	assert.Equal(t, int64(2), requestFailureMsg.responseTime)
	assert.Equal(t, "udp error", requestFailureMsg.error)

	defaultBoomer = &Boomer{}
}
