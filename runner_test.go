package boomer

import (
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
	if runner.numClients != 10 {
		t.Errorf("Number of goroutines mismatches, expected: 10, current count: %d\n", runner.numClients)
	}
}
