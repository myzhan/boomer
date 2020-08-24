package boomer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type HitOutput struct {
	onStart bool
	onEvent bool
	onStop  bool
}

func (o *HitOutput) OnStart() {
	o.onStart = true
}

func (o *HitOutput) OnEvent(data map[string]interface{}) {
	o.onEvent = true
}

func (o *HitOutput) OnStop() {
	o.onStop = true
}

func TestSafeRun(t *testing.T) {
	runner := &runner{}
	runner.safeRun(func() {
		panic("Runner will catch this panic")
	})
}

func TestOutputOnStart(t *testing.T) {
	hitOutput := &HitOutput{}
	hitOutput2 := &HitOutput{}
	runner := &runner{}
	runner.addOutput(hitOutput)
	runner.addOutput(hitOutput2)
	runner.outputOnStart()
	if !hitOutput.onStart {
		t.Error("hitOutput's OnStart has not been called")
	}
	if !hitOutput2.onStart {
		t.Error("hitOutput2's OnStart has not been called")
	}
}

func TestOutputOnEevent(t *testing.T) {
	hitOutput := &HitOutput{}
	hitOutput2 := &HitOutput{}
	runner := &runner{}
	runner.addOutput(hitOutput)
	runner.addOutput(hitOutput2)
	runner.outputOnEevent(nil)
	if !hitOutput.onEvent {
		t.Error("hitOutput's OnEvent has not been called")
	}
	if !hitOutput2.onEvent {
		t.Error("hitOutput2's OnEvent has not been called")
	}
}

func TestOutputOnStop(t *testing.T) {
	hitOutput := &HitOutput{}
	hitOutput2 := &HitOutput{}
	runner := &runner{}
	runner.addOutput(hitOutput)
	runner.addOutput(hitOutput2)
	runner.outputOnStop()
	if !hitOutput.onStop {
		t.Error("hitOutput's OnStop has not been called")
	}
	if !hitOutput2.onStop {
		t.Error("hitOutput2's OnStop has not been called")
	}
}

func TestLocalRunner(t *testing.T) {
	taskA := &Task{
		Weight: 10,
		Fn: func() {
			time.Sleep(time.Second)
		},
		Name: "TaskA",
	}
	tasks := []*Task{taskA}
	runner := newLocalRunner(tasks, nil, 2, 2)
	go runner.run()
	time.Sleep(4 * time.Second)
	runner.close()
}

func TestSpawnWorkers(t *testing.T) {
	taskA := &Task{
		Weight: 10,
		Fn: func() {
			time.Sleep(time.Second)
		},
		Name: "TaskA",
	}
	tasks := []*Task{taskA}

	runner := newSlaveRunner("localhost", 5557, tasks, nil)
	defer runner.close()

	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.spawnRate = 10

	go runner.spawnWorkers(10, runner.stopChan, runner.spawnComplete)
	time.Sleep(10 * time.Millisecond)

	currentClients := atomic.LoadInt32(&runner.numClients)
	if currentClients > 3 {
		t.Error("Spawning goroutines too fast, current count", currentClients)
	}
}

func TestSpawnWorkersWithManyTasks(t *testing.T) {
	var lock sync.Mutex
	taskCalls := map[string]int{}

	createTask := func(name string, weight int) *Task {
		return &Task{
			Name:   name,
			Weight: weight,
			Fn: func() {
				lock.Lock()
				defer lock.Unlock()
				taskCalls[name]++
			},
		}
	}
	tasks := []*Task{
		createTask(`one hundred`, 100),
		createTask(`ten`, 10),
		createTask(`one`, 1),
	}

	runner := newSlaveRunner("localhost", 5557, tasks, nil)
	defer runner.close()

	runner.client = newClient("localhost", 5557, runner.nodeID)

	const numToSpawn int = 30
	const spawnRate float64 = 10
	runner.spawnRate = spawnRate

	runner.spawnWorkers(numToSpawn, runner.stopChan, runner.spawnComplete)

	currentClients := atomic.LoadInt32(&runner.numClients)

	assert.Equal(t, numToSpawn, int(currentClients))
	lock.Lock()
	hundreds := taskCalls[`one hundred`]
	tens := taskCalls[`ten`]
	ones := taskCalls[`one`]
	lock.Unlock()

	total := hundreds + tens + ones
	t.Logf("total tasks run: %d\n", total)

	assert.True(t, total > 111)

	assert.True(t, ones > 1)
	actPercentage := float64(ones) / float64(total)
	expectedPercentage := 1.0 / 111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of ones task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, tens > 10)
	actPercentage = float64(tens) / float64(total)
	expectedPercentage = 10.0 / 111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of tens task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, hundreds > 100)
	actPercentage = float64(hundreds) / float64(total)
	expectedPercentage = 100.0 / 111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of hundreds task: exp %v, act %v", expectedPercentage, actPercentage)
	}
}

func TestSpawnWorkersWithManyTasksInWeighingTaskSet(t *testing.T) {
	var lock sync.Mutex
	taskCalls := map[string]int{}

	createTask := func(name string, weight int) *Task {
		return &Task{
			Name:   name,
			Weight: weight,
			Fn: func() {
				lock.Lock()
				defer lock.Unlock()
				taskCalls[name]++
			},
		}
	}

	wts := NewWeighingTaskSet()

	wts.AddTask(createTask(`one thousand`, 1000))
	wts.AddTask(createTask(`one hundred`, 100))
	wts.AddTask(createTask(`ten`, 10))
	wts.AddTask(createTask(`one`, 1))

	task := &Task{
		Name: "TaskSetWrapperTask",
		Fn:   wts.Run,
	}

	runner := newSlaveRunner("localhost", 5557, []*Task{task}, nil)
	defer runner.close()

	runner.client = newClient("localhost", 5557, runner.nodeID)

	const numToSpawn int = 30
	const spawnRate float64 = 10
	runner.spawnRate = spawnRate

	runner.spawnWorkers(numToSpawn, runner.stopChan, runner.spawnComplete)

	currentClients := atomic.LoadInt32(&runner.numClients)

	assert.Equal(t, numToSpawn, int(currentClients))
	lock.Lock()
	thousands := taskCalls[`one thousand`]
	hundreds := taskCalls[`one hundred`]
	tens := taskCalls[`ten`]
	ones := taskCalls[`one`]
	lock.Unlock()

	total := hundreds + tens + ones + thousands
	t.Logf("total tasks run: %d\n", total)

	assert.True(t, total > 1111)

	assert.True(t, ones > 1)
	actPercentage := float64(ones) / float64(total)
	expectedPercentage := 1.0 / 1111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of ones task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, tens > 10)
	actPercentage = float64(tens) / float64(total)
	expectedPercentage = 10.0 / 1111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of tens task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, hundreds > 100)
	actPercentage = float64(hundreds) / float64(total)
	expectedPercentage = 100.0 / 1111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of hundreds task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, thousands > 1000)
	actPercentage = float64(thousands) / float64(total)
	expectedPercentage = 1000.0 / 1111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of thousands task: exp %v, act %v", expectedPercentage, actPercentage)
	}
}

func TestSpawnAndStop(t *testing.T) {
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
	runner := newSlaveRunner("localhost", 5557, tasks, nil)
	defer runner.close()
	runner.client = newClient("localhost", 5557, runner.nodeID)

	go func() {
		var ticker = time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				t.Error("Timeout waiting for message sent by startSpawning()")
				return
			case <-runner.stats.clearStatsChan:
				// just quit
				return
			}
		}
	}()

	runner.startSpawning(10, float64(10), runner.spawnComplete)
	// wait for spawning goroutines
	time.Sleep(2 * time.Second)
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count", runner.numClients)
	}

	msg := <-runner.client.sendChannel()
	if msg.Type != "spawning_complete" {
		t.Error("Runner should send spawning_complete message when spawning completed, got", msg.Type)
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
	runner := newSlaveRunner("localhost", 5557, tasks, nil)
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

func TestOnSpawnMessage(t *testing.T) {
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}
	runner := newSlaveRunner("localhost", 5557, []*Task{taskA}, nil)
	defer runner.close()
	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.state = stateInit

	workers, spawnRate := 0, float64(0)
	callback := func(param1 int, param2 float64) {
		workers = param1
		spawnRate = param2
	}
	Events.Subscribe("boomer:spawn", callback)
	defer Events.Unsubscribe("boomer:spawn", callback)

	go func() {
		// consumes clearStatsChannel
		for {
			select {
			case <-runner.stats.clearStatsChan:
				return
			}
		}
	}()

	runner.onSpawnMessage(newMessage("spawn", map[string]interface{}{
		"spawn_rate": float64(20),
		"num_users":  int64(20),
	}, runner.nodeID))

	if workers != 20 {
		t.Error("workers should be overwrote by callback function, expected: 20, was:", workers)
	}
	if spawnRate != 20 {
		t.Error("spawnRate should be overwrote by callback function, expected: 20, was:", spawnRate)
	}

	runner.onMessage(newMessage("stop", nil, runner.nodeID))
}

func TestOnQuitMessage(t *testing.T) {
	runner := newSlaveRunner("localhost", 5557, nil, nil)
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

	runner := newSlaveRunner("localhost", 5557, tasks, nil)
	defer runner.close()
	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.state = stateInit

	go func() {
		// consumes clearStatsChannel
		count := 0
		for {
			select {
			case <-runner.stats.clearStatsChan:
				// receive two spawn message from master
				if count >= 2 {
					return
				}
				count++
			}
		}
	}()

	// start spawning
	runner.onMessage(newMessage("spawn", map[string]interface{}{
		"spawn_rate": float64(10),
		"num_users":  int64(10),
	}, runner.nodeID))

	msg := <-runner.client.sendChannel()
	if msg.Type != "spawning" {
		t.Error("Runner should send spawning message when starting spawn, got", msg.Type)
	}

	// spawn complete and running
	time.Sleep(2 * time.Second)
	if runner.state != stateRunning {
		t.Error("State of runner is not running after spawn, got", runner.state)
	}
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count:", runner.numClients)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "spawning_complete" {
		t.Error("Runner should send spawning_complete message when spawn completed, got", msg.Type)
	}

	// increase num_users while running
	runner.onMessage(newMessage("spawn", map[string]interface{}{
		"spawn_rate": float64(20),
		"num_users":  int64(20),
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
	if msg.Type != "spawning" {
		t.Error("Runner should send spawning message when starting spawn, got", msg.Type)
	}

	time.Sleep(2 * time.Second)
	if runner.state != stateRunning {
		t.Error("State of runner is not running after spawn, got", runner.state)
	}
	if runner.numClients != 20 {
		t.Error("Number of goroutines mismatches, expected: 20, current count:", runner.numClients)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "spawning_complete" {
		t.Error("Runner should send spawning_complete message when spawn completed, got", msg.Type)
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

	// spawn again
	runner.onMessage(newMessage("spawn", map[string]interface{}{
		"spawn_rate": float64(10),
		"num_users":  uint64(10),
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
	if msg.Type != "spawning" {
		t.Error("Runner should send spawning message when starting spawn, got", msg.Type)
	}

	// spawn complete and running
	time.Sleep(2 * time.Second)
	if runner.state != stateRunning {
		t.Error("State of runner is not running after spawn, got", runner.state)
	}
	if runner.numClients != 10 {
		t.Error("Number of goroutines mismatches, expected: 10, current count:", runner.numClients)
	}
	msg = <-runner.client.sendChannel()
	if msg.Type != "spawning_complete" {
		t.Error("Runner should send spawning_complete message when spawn completed, got", msg.Type)
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
	r := newSlaveRunner(masterHost, masterPort, nil, rateLimiter)
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
