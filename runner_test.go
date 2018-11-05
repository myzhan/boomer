package boomer

import (
	"runtime"
	"testing"
	"time"
)

func TestSafeRun(t *testing.T) {
	runner := &runner{}
	runner.safeRun(func() {
		panic("Runner will catch this panic")
	})
}

func TestGetReady(t *testing.T) {
	runner := &runner{
		nodeID: "TestingGetReady",
	}
	runner.getReady()
	if runner.state != stateInit {
		t.Error("The state of runner is not stateInit")
	}
	msg := <-toMaster
	t.Log(msg)
	if msg.Type != "client_ready" {
		t.Error("The msg type is not client_ready")
	}
	if msg.Data != nil {
		t.Error("The msg data is not nil")
	}
	if msg.NodeID != "TestingGetReady" {
		t.Error("The msg node_id is not TestingGetReady")
	}
}

func TestSpawnGoRoutines(t *testing.T) {
	taskA := &Task{
		Weight: 10,
		Fn: func() {
			time.Sleep(time.Second)
		},
		Name: "TaskA",
	}
	taskB := &Task{
		Weight: 20,
		Fn: func() {
			time.Sleep(2 * time.Second)
		},
		Name: "TaskB",
	}
	tasks := make([]*Task, 2)
	tasks[0] = taskA
	tasks[1] = taskB
	stopChannel := make(chan bool)
	runner := &runner{
		tasks:       tasks,
		numClients:  0,
		hatchRate:   10,
		stopChannel: stopChannel,
		nodeID:      "TestSpawnGoRoutines",
	}
	runner.spawnGoRoutines(10, runner.stopChannel)

	// Print out stack trace of all goroutines
	// buf := make([]byte, 1<<16)
	// runtime.Stack(buf, true)
	// fmt.Printf("%s", buf)

	// 3 goroutines spawned by testting and 12 goroutines spawned by boomer
	numGoroutine := runtime.NumGoroutine()
	// NOTICE: this test may be flaky.
	if numGoroutine != 15 {
		t.Errorf("Number of goroutines mismatches, expected: 15, current count: %d\n", numGoroutine)
	}
}
