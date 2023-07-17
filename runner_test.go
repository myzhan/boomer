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
	runner := newLocalRunner([]*Task{taskA}, nil, 2, 1)

	go runner.run()
	defer runner.shutdown()

	// wait for spawning
	time.Sleep(2100 * time.Millisecond)
	currentClients := atomic.LoadInt32(&runner.numClients)
	assert.Equal(t, int32(2), currentClients)
}

func TestLocalRunnerSendCustomMessage(t *testing.T) {
	Events.SubscribeOnce("TestLocalRunnerSendCustomMessage", func(customMessage *CustomMessage) {
		assert.Equal(t, "local", customMessage.NodeID)
		assert.Equal(t, "helloworld", customMessage.Data)
	})
	taskA := &Task{
		Weight: 10,
		Fn: func() {
			time.Sleep(time.Second)
		},
		Name: "TaskA",
	}
	runner := newLocalRunner([]*Task{taskA}, nil, 2, 2)
	runner.sendCustomMessage("TestLocalRunnerSendCustomMessage", "helloworld")
}

func TestSpawnWorkers(t *testing.T) {
	taskA := &Task{
		Weight: 10,
		Fn: func() {
			time.Sleep(time.Second)
		},
		Name: "TaskA",
	}

	runner := newSlaveRunner("localhost", 5557, []*Task{taskA}, nil)
	runner.client = newClient("localhost", 5557, runner.nodeID)
	defer runner.shutdown()

	go runner.spawnWorkers(10, runner.stopChan, runner.spawnComplete)
	time.Sleep(10 * time.Millisecond)

	currentClients := atomic.LoadInt32(&runner.numClients)
	assert.Equal(t, int32(10), currentClients)
}

func TestSpawnWorkersWithManyTasks(t *testing.T) {
	oneTaskCalls := int64(0)
	tenTaskCalls := int64(0)
	hundredTaskCalls := int64(0)

	oneTask := &Task{
		Name:   "one",
		Weight: 1,
		Fn: func() {
			atomic.AddInt64(&oneTaskCalls, 1)
		},
	}

	tenTask := &Task{
		Name:   "ten",
		Weight: 10,
		Fn: func() {
			atomic.AddInt64(&tenTaskCalls, 1)
		},
	}

	hundredTask := &Task{
		Name:   "hundred",
		Weight: 100,
		Fn: func() {
			atomic.AddInt64(&hundredTaskCalls, 1)
		},
	}

	runner := newSlaveRunner("localhost", 5557, []*Task{oneTask, tenTask, hundredTask}, nil)
	runner.client = newClient("localhost", 5557, runner.nodeID)
	defer runner.shutdown()

	const numToSpawn int = 30

	runner.spawnWorkers(numToSpawn, runner.stopChan, runner.spawnComplete)
	time.Sleep(3 * time.Second)

	currentClients := atomic.LoadInt32(&runner.numClients)
	assert.Equal(t, numToSpawn, int(currentClients))

	oneTaskActualCalls := atomic.LoadInt64(&oneTaskCalls)
	tenTaskActualCalls := atomic.LoadInt64(&tenTaskCalls)
	hundredTaskActualCalls := atomic.LoadInt64(&hundredTaskCalls)
	totalCalls := oneTaskActualCalls + tenTaskActualCalls + hundredTaskActualCalls

	assert.True(t, totalCalls > 111)
	assert.True(t, oneTaskActualCalls > 1)
	actPercentage := float64(oneTaskActualCalls) / float64(totalCalls)
	expectedPercentage := 1.0 / 111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of one task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, tenTaskActualCalls > 10)
	actPercentage = float64(tenTaskActualCalls) / float64(totalCalls)
	expectedPercentage = 10.0 / 111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of ten task: exp %v, act %v", expectedPercentage, actPercentage)
	}

	assert.True(t, hundredTaskActualCalls > 100)
	actPercentage = float64(hundredTaskActualCalls) / float64(totalCalls)
	expectedPercentage = 100.0 / 111.0
	if actPercentage > 2*expectedPercentage || actPercentage < 0.5*expectedPercentage {
		t.Errorf("Unexpected percentage of hundred task: exp %v, act %v", expectedPercentage, actPercentage)
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

	runner := newSlaveRunner("localhost", 5557, []*Task{taskA, taskB}, nil)
	runner.state = stateSpawning
	runner.client = newClient("localhost", 5557, runner.nodeID)
	defer runner.shutdown()
	runner.initSpawning()
	runner.startSpawning(10, float64(10), runner.spawnComplete)
	// wait for spawning goroutines
	time.Sleep(2 * time.Second)
	assert.Equal(t, int32(10), runner.numClients)

	msg := <-runner.client.sendChannel()
	m := msg.(*genericMessage)
	assert.Equal(t, "spawning_complete", m.Type)

	runner.stop()
	runner.onQuiting()

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "quit", m.Type)
	assert.Equal(t, runner.numClients, int32(0))
}

func TestStop(t *testing.T) {
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}

	runner := newSlaveRunner("localhost", 5557, []*Task{taskA}, nil)
	runner.initSpawning()
	stopped := false
	handler := func() {
		stopped = true
	}
	Events.Subscribe(EVENT_STOP, handler)
	defer Events.Unsubscribe(EVENT_STOP, handler)

	runner.stop()

	assert.True(t, stopped)
}

func TestOnSpawnMessage(t *testing.T) {
	taskA := &Task{
		Fn: func() {
			time.Sleep(time.Second)
		},
	}
	runner := newSlaveRunner("localhost", 5557, []*Task{taskA}, nil)
	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.state = stateInit
	defer runner.shutdown()

	workers, spawnRate := 0, float64(0)
	callback := func(param1 int, param2 float64) {
		workers = param1
		spawnRate = param2
	}
	Events.Subscribe(EVENT_SPAWN, callback)
	defer Events.Unsubscribe(EVENT_SPAWN, callback)

	runner.onSpawnMessage(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(10),
			"Dummy2": int64(10),
		},
		"timestamp": 1,
	}, runner.nodeID))

	assert.Equal(t, 20, workers)
	assert.Equal(t, float64(20), spawnRate)

	runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
}

func TestOnQuitMessage(t *testing.T) {
	runner := newSlaveRunner("localhost", 5557, nil, nil)
	runner.client = newClient("localhost", 5557, "test")
	runner.state = stateInit
	defer runner.shutdown()

	quitMessages := make(chan bool, 10)
	receiver := func() {
		quitMessages <- true
	}
	Events.Subscribe(EVENT_QUIT, receiver)
	defer Events.Unsubscribe(EVENT_QUIT, receiver)
	var ticker = time.NewTicker(20 * time.Millisecond)

	runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
	select {
	case <-quitMessages:
		break
	case <-ticker.C:
		t.Error("Runner should fire boomer:quit message when it receives a quit message from the master.")
		break
	}

	runner.state = stateRunning
	runner.stopChan = make(chan bool)
	runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
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
	runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
	select {
	case <-quitMessages:
		break
	case <-ticker.C:
		t.Error("Runner should fire boomer:quit message when it receives a quit message from the master.")
		break
	}

	assert.Equal(t, stateInit, runner.state)
}

func TestOnAckMessage(t *testing.T) {
	eventCount := 0
	Events.Subscribe(EVENT_CONNECTED, func() {
		eventCount++
	})
	runner := newSlaveRunner("localhost", 5557, []*Task{}, nil)
	runner.waitForAck = sync.WaitGroup{}
	runner.waitForAck.Add(1)

	runner.onAckMessage(nil)
	runner.onAckMessage(nil)
	assert.Equal(t, 1, eventCount)
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

	runner := newSlaveRunner("localhost", 5557, []*Task{taskA, taskB}, nil)
	runner.client = newClient("localhost", 5557, runner.nodeID)
	runner.state = stateInit
	defer runner.shutdown()

	go func() {
		// consumes clearStatsChannel
		count := 0
		for range runner.stats.clearStatsChan {
			// receive two spawn message from master
			if count >= 2 {
				return
			}
			count++
		}
	}()

	// start spawning
	runner.onMessage(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(5),
			"Dummy2": int64(5),
		},
	}, runner.nodeID))

	msg := <-runner.client.sendChannel()
	m := msg.(*genericMessage)
	assert.Equal(t, "spawning", m.Type)

	// spawn complete and running
	time.Sleep(2 * time.Second)
	assert.Equal(t, stateRunning, runner.state)
	assert.Equal(t, int32(10), runner.numClients)

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning_complete", m.Type)

	// increase goroutines while running
	runner.onMessage(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(10),
			"Dummy2": int64(10),
		},
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning", m.Type)

	time.Sleep(2 * time.Second)
	assert.Equal(t, stateRunning, runner.state)
	assert.Equal(t, int32(20), runner.numClients)

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning_complete", m.Type)

	// stop all the workers
	runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
	assert.Equal(t, stateInit, runner.state)
	assert.Equal(t, runner.numClients, int32(0))

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "client_stopped", m.Type)

	msg = <-runner.client.sendChannel()
	crm := msg.(*clientReadyMessage)
	assert.Equal(t, "client_ready", crm.Type)

	// spawn again
	runner.onMessage(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(5),
			"Dummy2": int64(5),
		},
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning", m.Type)

	// spawn complete and running
	time.Sleep(2 * time.Second)
	assert.Equal(t, stateRunning, runner.state)
	assert.Equal(t, int32(10), runner.numClients)

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning_complete", m.Type)

	// decrease goroutines while running
	runner.onMessage(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[interface{}]interface{}{
			"Dummy":  int64(3),
			"Dummy2": int64(2),
		},
	}, runner.nodeID))

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning", m.Type)

	// spawn complete and running
	time.Sleep(2 * time.Second)
	assert.Equal(t, stateRunning, runner.state)
	assert.Equal(t, int32(5), runner.numClients)

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "spawning_complete", m.Type)

	// stop all the workers
	runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
	assert.Equal(t, stateInit, runner.state)
	assert.Equal(t, runner.numClients, int32(0))

	msg = <-runner.client.sendChannel()
	m = msg.(*genericMessage)
	assert.Equal(t, "client_stopped", m.Type)

	msg = <-runner.client.sendChannel()
	crm = msg.(*clientReadyMessage)
	assert.Equal(t, "client_ready", crm.Type)
}

func TestGetReady(t *testing.T) {
	masterHost := "127.0.0.1"
	masterPort := 6557

	server := newTestServer(masterHost, masterPort)
	server.start()
	defer server.close()

	rateLimiter := NewStableRateLimiter(100, time.Second)
	r := newSlaveRunner(masterHost, masterPort, nil, rateLimiter)
	defer r.shutdown()
	defer Events.Unsubscribe(EVENT_QUIT, r.onQuiting)

	r.run()

	clientReady := <-server.fromClient
	crm := clientReady.(*clientReadyMessage)
	if crm.Type != "client_ready" {
		t.Error("Runner should send client_ready message to server.")
	}

	r.numClients = 10
	// it's not really running
	r.state = stateRunning
	data := make(map[string]interface{})
	r.stats.messageToRunnerChan <- data

	msg := <-server.fromClient
	m := msg.(*genericMessage)
	assert.Equal(t, "stats", m.Type)

	userCount := m.Data["user_count"].(int64)
	assert.Equal(t, int64(10), userCount)
}
