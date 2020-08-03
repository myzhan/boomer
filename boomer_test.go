package boomer

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBoomer(t *testing.T) {
	b := NewBoomer("0.0.0.0", 1234)

	if b.masterHost != "0.0.0.0" {
		t.Error("masterHost should be 0.0.0.0")
	}

	if b.masterPort != 1234 {
		t.Error("masterPort should be 1234")
	}

	if b.mode != DistributedMode {
		t.Error("mode should be DistributedMode")
	}
}

func TestNewStandaloneBoomer(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)

	if b.hatchCount != 100 {
		t.Error("hatchCount should be 100")
	}

	if b.hatchRate != 10 {
		t.Error("hatchRate should be 10")
	}

	if b.mode != StandaloneMode {
		t.Error("mode should be StandaloneMode")
	}
}

func TestSetRateLimiter(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	limiter, _ := NewRampUpRateLimiter(10, "10/1s", time.Second)
	b.SetRateLimiter(limiter)

	if b.rateLimiter == nil {
		t.Error("b.rateLimiter should not be nil")
	}
}

func TestSetMode(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)

	b.SetMode(DistributedMode)
	if b.mode != DistributedMode {
		t.Error("mode should be DistributedMode")
	}

	b.SetMode(StandaloneMode)
	if b.mode != StandaloneMode {
		t.Error("mode should be StandaloneMode")
	}

	b.SetMode(3)
	if b.mode != StandaloneMode {
		t.Error("mode should be StandaloneMode")
	}
}

func TestAddOutput(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	b.AddOutput(NewConsoleOutput())
	b.AddOutput(NewConsoleOutput())

	if len(b.outputs) != 2 {
		t.Error("length of outputs should be 2")
	}
}

func TestEnableCPUProfile(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	b.EnableCPUProfile("cpu.prof", time.Second)

	if b.cpuProfile != "cpu.prof" {
		t.Error("cpuProfile should be cpu.prof")
	}

	if b.cpuProfileDuration != time.Second {
		t.Error("cpuProfileDuration should 1 second")
	}
}

func TestEnableMemoryProfile(t *testing.T) {
	b := NewStandaloneBoomer(100, 10)
	b.EnableMemoryProfile("mem.prof", time.Second)

	if b.memoryProfile != "mem.prof" {
		t.Error("memoryProfile should be mem.prof")
	}

	if b.memoryProfileDuration != time.Second {
		t.Error("memoryProfileDuration should 1 second")
	}
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

	if count != 10 {
		t.Error("count is", count, "expected: 10")
	}

	if _, err := os.Stat("cpu.pprof"); os.IsNotExist(err) {
		t.Error("File cpu.pprof is not generated")
	} else {
		os.Remove("cpu.pprof")
	}

	if _, err := os.Stat("mem.pprof"); os.IsNotExist(err) {
		t.Error("File mem.pprof is not generated")
	} else {
		os.Remove("mem.pprof")
	}
}

func TestDistributedRun(t *testing.T) {
	masterHost := "0.0.0.0"
	rand.Seed(Now())
	masterPort := rand.Intn(1000) + 10240

	server := newTestServer(masterHost, masterPort)
	defer server.close()

	log.Println(fmt.Sprintf("Starting to serve on %s:%d", masterHost, masterPort))
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

	server.toClient <- newMessage("hatch", map[string]interface{}{
		"hatch_rate": float64(10),
		"num_users":  int64(10),
	}, b.slaveRunner.nodeID)

	time.Sleep(4 * time.Second)

	b.Quit()

	if count != 10 {
		t.Error("count is", count, "expected: 10")
	}
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

	if count != 1 {
		t.Error("count is", count, "expected: 1")
	}

	runTasks = ""
}

func TestRunTasksWithBoomerReport(t *testing.T) {
	taskA := &Task{
		Name: "report",
		Fn: func() {
			// it should not panic.
			RecordSuccess("http", "foo", int64(1), int64(10))
			RecordFailure("udp", "bar", int64(1), "udp error")
		},
	}
	runTasks = "report"

	runTasksForTest(taskA)

	runTasks = ""
}

func TestCreateRatelimiter(t *testing.T) {
	rateLimiter, _ := createRateLimiter(100, "-1")
	if stableRateLimiter, ok := rateLimiter.(*StableRateLimiter); !ok {
		t.Error("Expected stableRateLimiter")
	} else {
		if stableRateLimiter.threshold != 100 {
			t.Error("threshold should be equals to math.MaxInt64, was", stableRateLimiter.threshold)
		}
	}

	rateLimiter, _ = createRateLimiter(0, "1")
	if rampUpRateLimiter, ok := rateLimiter.(*RampUpRateLimiter); !ok {
		t.Error("Expected rampUpRateLimiter")
	} else {
		if rampUpRateLimiter.maxThreshold != math.MaxInt64 {
			t.Error("maxThreshold should be equals to math.MaxInt64, was", rampUpRateLimiter.maxThreshold)
		}
		if rampUpRateLimiter.rampUpRate != "1" {
			t.Error("rampUpRate should be equals to \"1\", was", rampUpRateLimiter.rampUpRate)
		}
	}

	rateLimiter, _ = createRateLimiter(10, "2/2s")
	if rampUpRateLimiter, ok := rateLimiter.(*RampUpRateLimiter); !ok {
		t.Error("Expected rampUpRateLimiter")
	} else {
		if rampUpRateLimiter.maxThreshold != 10 {
			t.Error("maxThreshold should be equals to 10, was", rampUpRateLimiter.maxThreshold)
		}
		if rampUpRateLimiter.rampUpRate != "2/2s" {
			t.Error("rampUpRate should be equals to \"2/2s\", was", rampUpRateLimiter.rampUpRate)
		}
		if rampUpRateLimiter.rampUpStep != 2 {
			t.Error("rampUpStep should be equals to 2, was", rampUpRateLimiter.rampUpStep)
		}
		if rampUpRateLimiter.rampUpPeroid != 2*time.Second {
			t.Error("rampUpPeroid should be equals to 2 seconds, was", rampUpRateLimiter.rampUpPeroid)
		}
	}
}

func TestRun(t *testing.T) {
	flag.Parse()

	masterHost = "0.0.0.0"
	rand.Seed(Now())
	masterPort = rand.Intn(1000) + 10240

	server := newTestServer(masterHost, masterPort)
	defer server.close()

	log.Println(fmt.Sprintf("Starting to serve on %s:%d", masterHost, masterPort))
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

	server.toClient <- newMessage("hatch", map[string]interface{}{
		"hatch_rate": float64(10),
		"num_users":  int64(10),
	}, defaultBoomer.slaveRunner.nodeID)

	time.Sleep(4 * time.Second)

	defaultBoomer.Quit()

	if count != 10 {
		t.Error("count is", count, "expected: 10")
	}
}

func TestSetInitTask(t *testing.T) {
	flag.Parse()

	masterHost = "0.0.0.0"
	rand.Seed(Now())
	masterPort = rand.Intn(1000) + 10240

	server := newTestServer(masterHost, masterPort)
	defer server.close()

	log.Println(fmt.Sprintf("Starting to serve on %s:%d", masterHost, masterPort))
	server.start()

	time.Sleep(20 * time.Millisecond)

	count := int64(0)
	taskInit := &Task{
		Name: "initTask",
		Fn: func() {
			atomic.AddInt64(&count, -1)
			runtime.Goexit()
		},
	}
	taskA := &Task{
		Name: "increaseCount",
		Fn: func() {
			atomic.AddInt64(&count, 1)
			runtime.Goexit()
		},
	}

	SetInitTask(taskInit)
	go Run(taskA)
	time.Sleep(20 * time.Millisecond)

	server.toClient <- newMessage("hatch", map[string]interface{}{
		"hatch_rate": float64(10),
		"num_users":  int64(10),
	}, defaultBoomer.slaveRunner.nodeID)

	time.Sleep(4 * time.Second)

	defaultBoomer.Quit()

	if count != 9 {
		t.Error("count is", count, "expected: 9")
	}
}

func TestRecordSuccess(t *testing.T) {
	masterHost := "127.0.0.1"
	masterPort := 5557
	defaultBoomer = NewBoomer(masterHost, masterPort)
	defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)
	RecordSuccess("http", "foo", int64(1), int64(10))

	requestSuccessMsg := <-defaultBoomer.slaveRunner.stats.requestSuccessChan
	if requestSuccessMsg.requestType != "http" {
		t.Error("Expected: http, got:", requestSuccessMsg.requestType)
	}
	if requestSuccessMsg.responseTime != int64(1) {
		t.Error("Expected: 1, got:", requestSuccessMsg.responseTime)
	}
	defaultBoomer = nil
}

func TestRecordFailure(t *testing.T) {
	masterHost := "127.0.0.1"
	masterPort := 5557
	defaultBoomer = NewBoomer(masterHost, masterPort)
	defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)
	RecordFailure("udp", "bar", int64(2), "udp error")

	requestFailureMsg := <-defaultBoomer.slaveRunner.stats.requestFailureChan
	if requestFailureMsg.requestType != "udp" {
		t.Error("Expected: udp, got:", requestFailureMsg.requestType)
	}
	if requestFailureMsg.responseTime != int64(2) {
		t.Error("Expected: 2, got:", requestFailureMsg.responseTime)
	}
	if requestFailureMsg.error != "udp error" {
		t.Error("Expected: udp error, got:", requestFailureMsg.error)
	}
	defaultBoomer = nil
}
