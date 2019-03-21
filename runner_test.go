package boomer

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSafeRun(t *testing.T) {
	runner := newRunner(nil, nil, "asap")
	runner.safeRun(func() {
		panic("Runner will catch this panic")
	})
}

func TestSpawnWorkers(t *testing.T) {
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
	rateLimiter := NewStableRateLimiter(100, time.Second)
	runner := newRunner(tasks, rateLimiter, "asap")
	defer runner.close()

	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.hatchRate = 10

	runner.spawnWorkers(10, runner.stopChan)
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count", runner.numClients)
	}
}

func TestSpawnWorkersSmoothly(t *testing.T) {
	taskA := &Task{
		Weight: 10,
		Fn: func() {
			time.Sleep(time.Second)
		},
		Name: "TaskA",
	}
	tasks := []*Task{taskA}

	runner := newRunner(tasks, nil, "smooth")
	defer runner.close()

	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.hatchRate = 10

	go runner.spawnWorkers(10, runner.stopChan)
	time.Sleep(2 * time.Millisecond)

	currentClients := atomic.LoadInt32(&runner.numClients)
	if currentClients > 3 {
		t.Error("Spawning goroutines too fast, current count", currentClients)
	}
}

func TestHatchAndStop(t *testing.T) {
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
	runner := newRunner(tasks, nil, "asap")
	defer runner.close()
	runner.client = newClient("localhost", 5557, runner.nodeID)

	go func() {
		var ticker = time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				t.Error("Timeout waiting for message sent by startHatching()")
				return
			case <-runner.stats.clearStatsChan:
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

	msg := <-runner.client.sendChannel()
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatching completed, got", msg.Type)
	}
	runner.stop()

	runner.onQuiting()
	msg = <-runner.client.sendChannel()
	if msg.Type != "quit" {
		t.Error("Runner should send quit message on quitting, got", msg.Type)
	}
}

func TestStop(t *testing.T) {
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}
	tasks := []*Task{taskA}
	runner := newRunner(tasks, nil, "asap")
	runner.stopChan = make(chan bool)

	stopped := false
	handler := func() {
		stopped = true
	}
	Events.Subscribe("boomer:stop", handler)
	defer Events.Unsubscribe("boomer:stop", handler)

	runner.stop()

	if stopped != true {
		t.Error("Expected stopped to be true, was", stopped)
	}
}

func TestOnHatchMessage(t *testing.T) {
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}
	runner := newRunner([]*Task{taskA}, nil, "asap")
	defer runner.close()
	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.state = stateInit

	workers, hatchRate := 0, 0
	callback := func(param1, param2 int) {
		workers = param1
		hatchRate = param2
	}
	Events.Subscribe("boomer:hatch", callback)
	defer Events.Unsubscribe("boomer:hatch", callback)

	go func() {
		// consumes clearStatsChannel
		for {
			select {
			case <-runner.stats.clearStatsChan:
				return
			}
		}
	}()

	runner.onHatchMessage(newMessage("hatch", map[string]interface{}{
		"hatch_rate":  float64(20),
		"num_clients": int64(20),
	}, runner.nodeID))

	if workers != 20 {
		t.Error("workers should be overwrote by callback function, expected: 20, was:", workers)
	}
	if hatchRate != 20 {
		t.Error("hatchRate should be overwrote by callback function, expected: 20, was:", hatchRate)
	}

	runner.onMessage(newMessage("stop", nil, runner.nodeID))
}

func TestOnQuitMessage(t *testing.T) {
	runner := newRunner(nil, nil, "asap")
	defer runner.close()
	runner.client = newClient("localhost", 5557, "test")
	runner.state = stateInit

	quitMessages := make(chan bool, 10)
	receiver := func() {
		quitMessages <- true
	}
	Events.Subscribe("boomer:quit", receiver)
	defer Events.Unsubscribe("boomer:quit", receiver)
	var ticker = time.NewTicker(20 * time.Millisecond)

	runner.onMessage(newMessage("quit", nil, runner.nodeID))
	select {
	case <-quitMessages:
		break
	case <-ticker.C:
		t.Error("Runner should fire boomer:quit message when it receives a quit message from the master.")
		break
	}

	runner.state = stateRunning
	runner.stopChan = make(chan bool)
	runner.onMessage(newMessage("quit", nil, runner.nodeID))
	select {
	case <-quitMessages:
		break
	case <-ticker.C:
		t.Error("Runner should fire boomer:quit message when it receives a quit message from the master.")
		break
	}
	if runner.state != stateInit {
		t.Error("Runner's state should be stateInit")
	}

	runner.state = stateStopped
	runner.onMessage(newMessage("quit", nil, runner.nodeID))
	select {
	case <-quitMessages:
		break
	case <-ticker.C:
		t.Error("Runner should fire boomer:quit message when it receives a quit message from the master.")
		break
	}
	if runner.state != stateInit {
		t.Error("Runner's state should be stateInit")
	}
}

func TestOnMessage(t *testing.T) {
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

	runner := newRunner(tasks, nil, "asap")
	defer runner.close()
	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.state = stateInit

	go func() {
		// consumes clearStatsChannel
		count := 0
		for {
			select {
			case <-runner.stats.clearStatsChan:
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

	msg := <-runner.client.sendChannel()
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
	msg = <-runner.client.sendChannel()
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatch completed, got", msg.Type)
	}

	// increase num_clients while running
	runner.onMessage(newMessage("hatch", map[string]interface{}{
		"hatch_rate":  float64(20),
		"num_clients": int64(20),
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
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
	msg = <-runner.client.sendChannel()
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatch completed, got", msg.Type)
	}

	// stop all the workers
	runner.onMessage(newMessage("stop", nil, runner.nodeID))
	if runner.state != stateInit {
		t.Error("State of runner is not init, got", runner.state)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "client_stopped" {
		t.Error("Runner should send client_stopped message, got", msg.Type)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "client_ready" {
		t.Error("Runner should send client_ready message, got", msg.Type)
	}

	// hatch again
	runner.onMessage(newMessage("hatch", map[string]interface{}{
		"hatch_rate":  float64(10),
		"num_clients": uint64(10),
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
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
	msg = <-runner.client.sendChannel()
	if msg.Type != "hatch_complete" {
		t.Error("Runner should send hatch_complete message when hatch completed, got", msg.Type)
	}

	// stop all the workers
	runner.onMessage(newMessage("stop", nil, runner.nodeID))
	if runner.state != stateInit {
		t.Error("State of runner is not init, got", runner.state)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "client_stopped" {
		t.Error("Runner should send client_stopped message, got", msg.Type)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "client_ready" {
		t.Error("Runner should send client_ready message, got", msg.Type)
	}
}

func TestGetReady(t *testing.T) {
	masterHost := "127.0.0.1"
	masterPort := 6557

	server := newTestServer(masterHost, masterPort)
	defer server.close()
	server.start()

	rateLimiter := NewStableRateLimiter(100, time.Second)
	r := newRunner(nil, rateLimiter, "asap")
	r.masterHost = masterHost
	r.masterPort = masterPort
	defer r.close()
	defer Events.Unsubscribe("boomer:quit", r.onQuiting)

	r.run()

	msg := <-server.fromClient
	if msg.Type != "client_ready" {
		t.Error("Runner should send client_ready message to server.")
	}

	r.numClients = 10
	// it's not really running
	r.state = stateRunning
	data := make(map[string]interface{})
	r.stats.messageToRunnerChan <- data

	msg = <-server.fromClient
	if msg.Type != "stats" {
		t.Error("Runner should send stats message to server.")
	}

	userCount := msg.Data["user_count"].(int64)
	if userCount != int64(10) {
		t.Error("User count mismatch, expect: 10, got:", userCount)
	}
}
