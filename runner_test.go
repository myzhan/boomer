package boomer

import (
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/myzhan/gomq/zmtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("Test runner", func() {

	It("test saferun", func() {
		Expect(func() {
			runner := &runner{}
			runner.setLogger(log.Default())
			runner.safeRun(func() {
				panic("Runner will catch this panic")
			})
		}).Should(Not(Panic()))
	})

	It("test output onStart", func() {
		hitOutput := &HitOutput{}
		hitOutput2 := &HitOutput{}
		runner := &runner{}
		runner.setLogger(log.Default())
		runner.addOutput(hitOutput)
		runner.addOutput(hitOutput2)
		runner.outputOnStart()
		Expect(hitOutput.onStart).To(BeTrue())
		Expect(hitOutput2.onStart).To(BeTrue())
	})

	It("test output onEvent", func() {
		hitOutput := &HitOutput{}
		hitOutput2 := &HitOutput{}
		runner := &runner{}
		runner.setLogger(log.Default())
		runner.addOutput(hitOutput)
		runner.addOutput(hitOutput2)
		runner.outputOnEvent(nil)
		Expect(hitOutput.onEvent).To(BeTrue())
		Expect(hitOutput2.onEvent).To(BeTrue())
	})

	It("test output onStop", func() {
		hitOutput := &HitOutput{}
		hitOutput2 := &HitOutput{}
		runner := &runner{}
		runner.setLogger(log.Default())
		runner.addOutput(hitOutput)
		runner.addOutput(hitOutput2)
		runner.outputOnStop()
		Expect(hitOutput.onStop).To(BeTrue())
		Expect(hitOutput2.onStop).To(BeTrue())
	})

	It("test add workers", func() {
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

		runner.addWorkers(10)

		runner.mu.RLock()
		currentClients := len(runner.cancelFuncs)
		runner.mu.RUnlock()
		Expect(currentClients).To(BeEquivalentTo(10))
	})

	It("test reduce workers", func() {
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

		runner.addWorkers(10)
		runner.reduceWorkers(5)
		runner.reduceWorkers(2)

		runner.mu.RLock()
		currentClients := len(runner.cancelFuncs)
		runner.mu.RUnlock()
		Expect(currentClients).To(BeEquivalentTo(3))
	})

	It("test localrunner", func() {
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
		Expect(currentClients).To(BeEquivalentTo(2))
	})

	It("test local runner send custom message", func() {
		Events.SubscribeOnce("TestLocalRunnerSendCustomMessage", func(customMessage *CustomMessage) {
			Expect(customMessage.NodeID).To(Equal("local"))
			Expect(customMessage.Data).To(Equal("helloworld"))
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
	})

	It("test spawn workers", func() {
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

		runner.spawnWorkers(10, runner.spawnComplete)

		currentClients := atomic.LoadInt32(&runner.numClients)
		Expect(currentClients).To(BeEquivalentTo(10))
	})

	It("test spawn workers with many tasks", func() {
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

		runner.spawnWorkers(numToSpawn, runner.spawnComplete)
		time.Sleep(3 * time.Second)

		currentClients := atomic.LoadInt32(&runner.numClients)
		Expect(currentClients).To(BeEquivalentTo(numToSpawn))

		oneTaskActualCalls := atomic.LoadInt64(&oneTaskCalls)
		tenTaskActualCalls := atomic.LoadInt64(&tenTaskCalls)
		hundredTaskActualCalls := atomic.LoadInt64(&hundredTaskCalls)
		totalCalls := oneTaskActualCalls + tenTaskActualCalls + hundredTaskActualCalls

		Expect(totalCalls).To(BeNumerically(">", 111))
		Expect(oneTaskActualCalls).To(BeNumerically(">", 1))

		actPercentage := float64(oneTaskActualCalls) / float64(totalCalls)
		expectedPercentage := 1.0 / 111.0

		Expect(actPercentage).To(BeNumerically("<", 2*expectedPercentage))
		Expect(actPercentage).To(BeNumerically(">", 0.5*expectedPercentage))
		Expect(tenTaskActualCalls).To(BeNumerically(">", 10))

		actPercentage = float64(tenTaskActualCalls) / float64(totalCalls)
		expectedPercentage = 10.0 / 111.0

		Expect(actPercentage).To(BeNumerically("<", 2*expectedPercentage))
		Expect(actPercentage).To(BeNumerically(">", 0.5*expectedPercentage))
		Expect(hundredTaskActualCalls).To(BeNumerically(">", 100))

		actPercentage = float64(hundredTaskActualCalls) / float64(totalCalls)
		expectedPercentage = 100.0 / 111.0
		Expect(actPercentage).To(BeNumerically("<", 2*expectedPercentage))
		Expect(actPercentage).To(BeNumerically(">", 0.5*expectedPercentage))
	})

	It("test spawn and stop", func() {
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
		runner.setState(stateSpawning)
		runner.client = newClient("localhost", 5557, runner.nodeID)
		defer runner.shutdown()

		runner.startSpawning(10, float64(10), runner.spawnComplete)
		// wait for spawning goroutines
		time.Sleep(2 * time.Second)
		Expect(atomic.LoadInt32(&runner.numClients)).To(BeEquivalentTo(10))

		msg := <-runner.client.sendChannel()
		m := msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))

		runner.stop()
		runner.onQuiting()

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("quit"))
	})

	It("test stop", func() {
		taskA := &Task{
			Fn: func() {
				time.Sleep(time.Second)
			},
		}

		runner := newSlaveRunner("localhost", 5557, []*Task{taskA}, nil)

		stopped := false
		handler := func() {
			stopped = true
		}
		Events.Subscribe(EVENT_STOP, handler)
		defer Events.Unsubscribe(EVENT_STOP, handler)

		runner.stop()

		Expect(stopped).To(BeTrue())
	})

	It("test on spawn message", func() {
		taskA := &Task{
			Fn: func() {
				time.Sleep(time.Second)
			},
		}
		runner := newSlaveRunner("localhost", 5557, []*Task{taskA}, nil)
		runner.client = newClient("localhost", 5557, runner.nodeID)
		runner.setState(stateInit)
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

		Expect(workers).To(BeEquivalentTo(20))
		Expect(spawnRate).To(BeEquivalentTo(20))
		runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
	})

	It("test onQuitMessage", func() {
		runner := newSlaveRunner("localhost", 5557, nil, nil)
		runner.client = newClient("localhost", 5557, "test")
		runner.setState(stateInit)
		defer runner.shutdown()

		quitMessages := make(chan bool, 10)
		receiver := func() {
			quitMessages <- true
		}
		Events.Subscribe(EVENT_QUIT, receiver)
		defer Events.Unsubscribe(EVENT_QUIT, receiver)

		runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
		Eventually(quitMessages).Should(Receive())

		runner.setState(stateRunning)
		runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
		Eventually(quitMessages).Should(Receive())
		Expect(runner.getState()).Should(BeIdenticalTo(stateInit))

		runner.setState(stateStopped)
		runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
		Eventually(quitMessages).Should(Receive())
		Expect(runner.getState()).Should(BeIdenticalTo(stateInit))
	})

	It("test on ack message", func() {
		eventCount := 0
		Events.Subscribe(EVENT_CONNECTED, func() {
			eventCount++
		})
		runner := newSlaveRunner("localhost", 5557, []*Task{}, nil)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		runner.waitForAck = wg

		runner.onAckMessage(nil)
		runner.onAckMessage(nil)
		Expect(eventCount).To(BeEquivalentTo(1))
	})

	It("test on message", func() {
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
		runner.setState(stateInit)
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
		Expect(m.Type).To(Equal("spawning"))

		time.Sleep(2 * time.Second)
		Expect(runner.getState()).Should(BeIdenticalTo(stateRunning))
		Expect(atomic.LoadInt32(&runner.numClients)).Should(BeEquivalentTo(10))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))
		// locust stores user_classes_count as is, and expects a dict.
		Expect(m.Data["user_classes_count"]).To(Equal(map[string]int64{"Dummy": 5, "Dummy2": 5}))

		// increase goroutines while running
		runner.onMessage(newGenericMessage("spawn", map[string]interface{}{
			"user_classes_count": map[interface{}]interface{}{
				"Dummy":  int64(10),
				"Dummy2": int64(10),
			},
		}, runner.nodeID))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning"))

		time.Sleep(2 * time.Second)
		Expect(runner.getState()).Should(BeIdenticalTo(stateRunning))
		Expect(atomic.LoadInt32(&runner.numClients)).Should(BeEquivalentTo(20))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))

		// stop all the workers
		runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
		Expect(runner.getState()).To(BeIdenticalTo(stateInit))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("client_stopped"))

		msg = <-runner.client.sendChannel()
		crm := msg.(*clientReadyMessage)
		Expect(crm.Type).To(Equal("client_ready"))
		// -1 bypasses the version check of locust master.
		Expect(crm.Data).To(BeEquivalentTo(-1))

		// spawn again
		runner.onMessage(newGenericMessage("spawn", map[string]interface{}{
			"user_classes_count": map[interface{}]interface{}{
				"Dummy":  int64(5),
				"Dummy2": int64(5),
			},
		}, runner.nodeID))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning"))

		// spawn complete and running
		time.Sleep(2 * time.Second)
		Expect(runner.getState()).Should(BeIdenticalTo(stateRunning))
		Expect(atomic.LoadInt32(&runner.numClients)).Should(BeEquivalentTo(10))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))
		// stop all the workers
		runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
		Expect(runner.getState()).To(BeIdenticalTo(stateInit))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("client_stopped"))

		msg = <-runner.client.sendChannel()
		crm = msg.(*clientReadyMessage)
		Expect(crm.Type).To(Equal("client_ready"))
	})

	It("test get ready", func() {
		masterHost := "mock:127.0.0.1"
		masterPort := 6557

		rateLimiter := NewStableRateLimiter(100, time.Second)
		r := newSlaveRunner(masterHost, masterPort, nil, rateLimiter)
		defer r.shutdown()
		defer Events.Unsubscribe(EVENT_QUIT, r.onQuiting)

		r.run()

		clientReadyMessage := newClientReadyMessage("client_ready", -1, r.nodeID)
		clientReadyMessageInBytes, _ := clientReadyMessage.serialize()
		Eventually(mockGomqDealerInstance.SendChannel()).Should(Receive(Equal(clientReadyMessageInBytes)))

		ackMessage := newGenericMessage("ack", nil, r.nodeID)
		ackMessageInBytes, _ := ackMessage.serialize()
		ackZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{ackMessageInBytes},
		}
		mockGomqDealerInstance.RecvChannel() <- ackZmtpMessage
	})
})

func newTestSlaveRunner(tasks ...*Task) *slaveRunner {
	r := newSlaveRunner("mock:0.0.0.0", 1234, tasks, nil)
	r.setLogger(newDiscardLogger())
	return r
}

func noopTask(name string) *Task {
	return &Task{Name: name, Weight: 1, Fn: func() {}}
}

// Locust before 2.0 doesn't send user_classes_count at all, it used to panic with
// "interface conversion: interface {} is nil".
func TestSpawnMessageWithoutUserClassesCount(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	defer r.shutdown()
	r.stats.start()

	r.onMessage(newGenericMessage("spawn", map[string]interface{}{"timestamp": 1}, r.nodeID))

	if workers := r.sumUsersAmount(newGenericMessage("spawn", map[string]interface{}{}, r.nodeID)); workers != 0 {
		t.Fatalf("expected 0 workers, got %d", workers)
	}
	if workers := r.sumUsersAmount(nil); workers != 0 {
		t.Fatalf("expected 0 workers for a nil message, got %d", workers)
	}
}

func TestSpawnMessageWithStringKeys(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	defer r.shutdown()

	workers := r.sumUsersAmount(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": map[string]interface{}{
			"Dummy":  int64(3),
			"Dummy2": int64(4),
		},
	}, r.nodeID))

	if workers != 7 {
		t.Fatalf("expected 7 workers, got %d", workers)
	}
}

func TestSpawnMessageWithInvalidUserClassesCount(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	defer r.shutdown()

	workers := r.sumUsersAmount(newGenericMessage("spawn", map[string]interface{}{
		"user_classes_count": "not-a-map",
	}, r.nodeID))

	if workers != 0 {
		t.Fatalf("expected 0 workers, got %d", workers)
	}
}

// A custom message with the type of a generic message used to be dereferenced as
// a genericMessage, which panicked with a nil pointer dereference.
func TestOnMessageWithCustomSpawnMessage(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	defer r.shutdown()

	r.setState(stateInit)
	r.onMessage(newCustomMessage("spawn", nil, r.nodeID))
	r.onMessage(newCustomMessage("ack", nil, r.nodeID))
}

// Spawning workers without tasks used to panic with "integer divide by zero".
func TestSpawnWorkersWithoutTasks(t *testing.T) {
	r := newTestSlaveRunner()
	defer r.shutdown()

	completed := make(chan struct{}, 1)
	r.spawnWorkers(5, func() {
		completed <- struct{}{}
	})

	select {
	case <-completed:
	case <-time.After(5 * time.Second):
		t.Fatal("spawnCompleteFunc was not called when there are no tasks")
	}

	if clients := atomic.LoadInt32(&r.numClients); clients != 0 {
		t.Fatalf("expected 0 clients, got %d", clients)
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	slave := newTestSlaveRunner(noopTask("noop"))
	slave.shutdown()
	// Shutting down twice used to panic with "close of closed channel".
	slave.shutdown()

	local := newLocalRunner([]*Task{noopTask("noop")}, nil, 1, 1)
	local.setLogger(newDiscardLogger())
	local.shutdown()
	local.shutdown()
}

// A full send channel used to block the caller forever once the runner is shutting down.
func TestSendMessageGivesUpAfterShutdown(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	c := newClient("mock:0.0.0.0", 1234, r.nodeID)
	r.client = c

	// Fill the channel so that sending can only succeed by giving up.
	blocker := newGenericMessage("ping", nil, r.nodeID)
	for i := 0; i < cap(c.sendChannel()); i++ {
		c.sendChannel() <- blocker
	}

	r.shutdown()

	sent := make(chan struct{})
	go func() {
		r.sendMessage(newGenericMessage("stats", nil, r.nodeID))
		close(sent)
	}()

	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("sendMessage blocked after shutdown")
	}
}

// A late spawning_complete must not move a stopped runner back to running.
func TestSpawnCompleteKeepsStoppedState(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	defer r.shutdown()

	r.setState(stateStopped)
	r.spawnComplete()

	if state := r.getState(); state != stateStopped {
		t.Fatalf("expected state %q, got %q", stateStopped, state)
	}
}

func TestOnQuitingSendsQuitOnlyOnce(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	client := newRecordingClient()
	r.client = client
	defer r.shutdown()

	r.onQuiting()
	r.onQuiting()

	if queued := len(client.toMaster); queued != 1 {
		t.Fatalf("expected 1 queued quit message, got %d", queued)
	}
	if state := r.getState(); state != stateQuitting {
		t.Fatalf("expected state %q, got %q", stateQuitting, state)
	}

	// locust master expects a quit message without data, tagged with our node id.
	quit, ok := nextMessage(t, client.toMaster, time.Second).(*genericMessage)
	if !ok || quit.Type != "quit" {
		t.Fatalf("expected a quit message, got %v", quit)
	}
	if quit.NodeID != r.nodeID {
		t.Fatalf("expected node_id %q, got %q", r.nodeID, quit.NodeID)
	}
	if quit.Data != nil {
		t.Fatalf("expected no data in a quit message, got %v", quit.Data)
	}
}

// Without EVENT_QUIT boomer.Run() blocks forever after master heartbeat timeout.
func TestHeartbeatTimeoutPublishesQuitEvent(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	defer r.shutdown()

	quitEvents := make(chan struct{}, 1)
	handler := func() {
		select {
		case quitEvents <- struct{}{}:
		default:
		}
	}
	Events.Subscribe(EVENT_QUIT, handler)
	defer Events.Unsubscribe(EVENT_QUIT, handler)

	r.startHeartbeat()
	// Simulate a master which has been quiet for longer than the timeout.
	r.lastMasterHeartbeatTimestamp.Store(time.Now().Add(-masterHeartbeatTimeout - time.Minute))

	select {
	case <-quitEvents:
	case <-time.After(5 * time.Second):
		t.Fatal("EVENT_QUIT was not published after master heartbeat timeout")
	}
}

func TestGetTaskOutOfRange(t *testing.T) {
	r := &runner{runTask: []*Task{{Name: "only", Weight: 1, Fn: func() {}}}}

	if task := r.getTask(-1); task != nil {
		t.Fatalf("expected nil for a negative index, got %v", task)
	}
	if task := r.getTask(1); task != nil {
		t.Fatalf("expected nil for an index out of range, got %v", task)
	}
	if task := r.getTask(0); task == nil {
		t.Fatal("expected the task at a valid index")
	}

	empty := &runner{}
	if task := empty.nextTaskIndex(); task != 0 {
		t.Fatalf("expected 0 for a runner without tasks, got %d", task)
	}
}

type panickingOutput struct{}

func (o *panickingOutput) OnStart()                            { panic("output OnStart panic") }
func (o *panickingOutput) OnEvent(data map[string]interface{}) { panic("output OnEvent panic") }
func (o *panickingOutput) OnStop()                             { panic("output OnStop panic") }

// A panic in a user defined Output must not crash boomer nor hang the caller.
func TestOutputPanicsAreIsolated(t *testing.T) {
	r := &runner{}
	r.setLogger(newDiscardLogger())
	r.addOutput(&panickingOutput{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.outputOnStart()
		r.outputOnEvent(nil)
		r.outputOnStop()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a panicking output hung the runner")
	}
}

// recordingClient is an in-memory client, it records everything the runner sends
// to master so that the wire protocol can be asserted without a real connection.
type recordingClient struct {
	toMaster               chan message
	fromMaster             chan message
	disconnectedFromMaster chan bool
	shutdownChan           chan bool
}

func newRecordingClient() *recordingClient {
	return &recordingClient{
		toMaster:               make(chan message, 100),
		fromMaster:             make(chan message, 100),
		disconnectedFromMaster: make(chan bool, 1),
		shutdownChan:           make(chan bool),
	}
}

func (c *recordingClient) connect() (err error)           { return nil }
func (c *recordingClient) close()                         { close(c.shutdownChan) }
func (c *recordingClient) recvChannel() chan message      { return c.fromMaster }
func (c *recordingClient) sendChannel() chan message      { return c.toMaster }
func (c *recordingClient) disconnectedChannel() chan bool { return c.disconnectedFromMaster }

func nextMessage(t *testing.T, ch chan message, timeout time.Duration) message {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for a message to master")
		return nil
	}
}

func TestHeartbeatTimeoutWithFullSendQueue(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	c := newRecordingClient()
	r.client = c
	r.setState(stateRunning)
	for i := 0; i < cap(c.toMaster); i++ {
		c.toMaster <- newGenericMessage("stats", nil, r.nodeID)
	}
	Events.Subscribe(EVENT_QUIT, r.onQuiting)
	quit := make(chan struct{}, 1)
	onQuit := func() { quit <- struct{}{} }
	Events.Subscribe(EVENT_QUIT, onQuit)
	defer func() {
		// Release a blocked quit send as well if the regression returns.
		<-c.toMaster
		r.shutdown()
		Events.Unsubscribe(EVENT_QUIT, onQuit)
	}()
	r.lastMasterHeartbeatTimestamp.Store(time.Now().Add(-2 * masterHeartbeatTimeout))
	r.startHeartbeat()
	select {
	case <-quit:
	case <-time.After(5 * time.Second):
		t.Fatal("heartbeat timeout blocked on the full send queue")
	}
	select {
	case <-r.shutdownChan:
	default:
		t.Fatal("quit event published before local shutdown")
	}
}

// locust master reads state, current_cpu_usage and current_memory_usage from the
// heartbeat message.
func TestHeartbeatMessageMatchesLocust(t *testing.T) {
	r := newTestSlaveRunner(noopTask("noop"))
	client := newRecordingClient()
	r.client = client
	defer r.shutdown()

	r.setState(stateRunning)
	r.startHeartbeat()

	heartbeat, ok := nextMessage(t, client.toMaster, 5*time.Second).(*genericMessage)
	if !ok || heartbeat.Type != "heartbeat" {
		t.Fatalf("expected a heartbeat message, got %v", heartbeat)
	}
	if heartbeat.Data["state"] != stateRunning {
		t.Fatalf("expected state %q, got %v", stateRunning, heartbeat.Data["state"])
	}
	if _, ok := heartbeat.Data["current_cpu_usage"]; !ok {
		t.Fatal("expected current_cpu_usage in the heartbeat message")
	}
	if _, ok := heartbeat.Data["current_memory_usage"]; !ok {
		t.Fatal("expected current_memory_usage in the heartbeat message")
	}
}
