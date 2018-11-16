package boomer

import (
	"os"
	"testing"
	"time"
)

func TestInitBoomer(t *testing.T) {
	initBoomer()
	defer Events.Unsubscribe("request_success", requestSuccessHandler)
	defer Events.Unsubscribe("request_failure", requestFailureHandler)
	defer defaultStats.close()

	defer func() {
		err := recover()
		if err == nil {
			t.Error("It should panic if initBoomer is called more than once.")
		}
	}()
	initBoomer()
}

func TestRunTasksForTest(t *testing.T) {
	count := 0
	taskA := &Task{
		Name: "increaseCount",
		Fn: func() {
			count++
		},
	}
	runTasks = "increaseCount,foobar"
	runTasksForTest(taskA)

	if count != 1 {
		t.Error("count is", count, "expected: 1")
	}
}

func TestStartMemoryProfile(t *testing.T) {
	if _, err := os.Stat("mem.pprof"); os.IsExist(err) {
		os.Remove("mem.pprof")
	}
	startMemoryProfile("mem.pprof", 3*time.Second)
	time.Sleep(4 * time.Second)
	if _, err := os.Stat("mem.pprof"); os.IsNotExist(err) {
		t.Error("File mem.pprof is not generated")
	} else {
		os.Remove("mem.pprof")
	}
}

func TestStartCPUProfile(t *testing.T) {
	if _, err := os.Stat("cpu.pprof"); os.IsExist(err) {
		os.Remove("cpu.pprof")
	}
	startCPUProfile("cpu.pprof", 3*time.Second)
	time.Sleep(4 * time.Second)
	if _, err := os.Stat("cpu.pprof"); os.IsNotExist(err) {
		t.Error("File cpu.pprof is not generated")
	} else {
		os.Remove("cpu.pprof")
	}
}
