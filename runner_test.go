package boomer

import (
	"testing"
	"time"
)

func TestParseRPSControlArgs(t *testing.T) {
	r := newRunner(nil, int64(100), "100/2s")
	r.parseRPSControlArgs()

	if !r.rpsControlEnabled {
		t.Error("r.rpsControlEnabled is, expected: true", r.rpsControlEnabled)
	}
	if r.requestIncreaseStep != 100 {
		t.Error("r.requestIncreaseStep is", r.requestIncreaseStep, "expected:", 100)
	}
	if r.requestIncreaseInterval != 2*time.Second {
		t.Error("requestIncreaseInterval is", r.requestIncreaseInterval, "expected: 2s")
	}

	// parse again
	r.requestIncreaseRate = "200"
	r.parseRPSControlArgs()
	if r.requestIncreaseStep != 200 {
		t.Error("r.requestIncreaseStep is", r.requestIncreaseStep, "expected:", 200)
	}
	if r.requestIncreaseInterval != time.Second {
		t.Error("r.requestIncreaseInterval is", r.requestIncreaseInterval, "expected: 1s")
	}
}

func TestSafeRun(t *testing.T) {
	runner := &runner{}
	runner.safeRun(func() {
		panic("Runner will catch this panic")
	})
}

func TestSpawnGoRoutines(t *testing.T) {
	// FIXME: recreate global toMaster channel, this will make this test flaky
	toMaster = make(chan *message, 10)
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
	tasks := []*Task{taskA, taskB}
	stopChannel := make(chan bool)
	runner := &runner{
		tasks:       tasks,
		stopChannel: stopChannel,
		nodeID:      "TestSpawnGoRoutines",
	}
	runner.hatchRate = 10
	runner.spawnGoRoutines(10, runner.stopChannel)
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count", runner.numClients)
	}
	close(toMaster)
}

func TestHatchAndStop(t *testing.T) {
	// FIXME: recreate global toMaster channel, this will make this test flaky
	toMaster = make(chan *message, 20)
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}
	taskB := &Task{
		Fn: func() {
			time.Sleep(2 * time.Second)
		},
	}
	tasks := []*Task{taskA, taskB}
	stopChannel := make(chan bool)
	runner := &runner{
		tasks:       tasks,
		stopChannel: stopChannel,
		nodeID:      "TestHatchAndStop",
	}

	go func() {
		var ticker = time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				t.Error("Timeout waiting for message sent by startHatching()")
				return
			case <-defaultStats.clearStatsChannel:
				// just quit
				return
			}
		}
	}()

	runner.startHatching(10, 10)
	// wait for spawning goroutines
	time.Sleep(100 * time.Millisecond)
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count", runner.numClients)
	}

	msg := <-toMaster
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatching completed, got", msg.Type)
	}
	runner.stop()

	runner.onQuiting()
	msg = <-toMaster
	if msg.Type != "quit" {
		t.Error("Runner should send quit message on quitting, got", msg.Type)
	}

	close(toMaster)
}

func TestOnMessage(t *testing.T) {
	// FIXME: recreate global toMaster channel, this will make this test flaky
	toMaster = make(chan *message, 20)
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}
	taskB := &Task{
		Fn: func() {
			time.Sleep(2 * time.Second)
		},
	}
	tasks := []*Task{taskA, taskB}
	stopChannel := make(chan bool)
	runner := &runner{
		tasks:       tasks,
		stopChannel: stopChannel,
		nodeID:      "TestOnMessage",
	}
	runner.state = stateInit

	go func() {
		// consumes clearStatsChannel
		count := 0
		for {
			select {
			case <-defaultStats.clearStatsChannel:
				// receive two hatch message from master
				if count >= 2 {
					return
				}
				count++
			}
		}
	}()

	// start hatching
	runner.onMessage(newMessage("hatch", map[string]interface{}{
		"hatch_rate":  float64(10),
		"num_clients": int64(10),
	}, runner.nodeID))

	msg := <-toMaster
	if msg.Type != "hatching" {
		t.Error("Runner should send hatching message when starting hatch, got", msg.Type)
	}

	// hatch complete and running
	time.Sleep(100 * time.Millisecond)
	if runner.state != stateRunning {
		t.Error("State of runner is not running after hatch, got", runner.state)
	}
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count:", runner.numClients)
	}
	msg = <-toMaster
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatch completed, got", msg.Type)
	}

	// increase num_clients while running
	runner.onMessage(newMessage("hatch", map[string]interface{}{
		"hatch_rate":  float64(20),
		"num_clients": int64(20),
	}, runner.nodeID))

	msg = <-toMaster
	if msg.Type != "hatching" {
		t.Error("Runner should send hatching message when starting hatch, got", msg.Type)
	}

	time.Sleep(100 * time.Millisecond)
	if runner.state != stateRunning {
		t.Error("State of runner is not running after hatch, got", runner.state)
	}
	if runner.numClients != 20 {
		t.Error("Number of goroutines mismatches, expected: 20, current count:", runner.numClients)
	}
	msg = <-toMaster
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatch completed, got", msg.Type)
	}

	// stop all the workers
	runner.onMessage(newMessage("stop", nil, runner.nodeID))
	if runner.state != stateStopped {
		t.Error("State of runner is not stopped, got", runner.state)
	}
	msg = <-toMaster
	if msg.Type != "client_stopped" {
		t.Error("Runner should send client_stopped message, got", msg.Type)
	}
	msg = <-toMaster
	if msg.Type != "client_ready" {
		t.Error("Runner should send client_ready message, got", msg.Type)
	}

	close(toMaster)
}

func TestStartBucketUpdater(t *testing.T) {
	r := newRunner(nil, 100, "-1")
	r.parseRPSControlArgs()
	r.startBucketUpdater()

	defer func() {
		close(r.rpsControllerQuitChannel)
	}()

	time.Sleep(1 * time.Second)
	if r.rpsThreshold != r.currentRPSThreshold {
		t.Error("rpsThreshold is not updated by bucket updater, expected", r.currentRPSThreshold, ", but got", r.rpsThreshold)
	}
}
