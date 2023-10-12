package boomer

import (
	"log"
	"sync"
	"sync/atomic"
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
		runner.outputOnEevent(nil)
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

		currentClients := len(runner.cancelFuncs)
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

		currentClients := len(runner.cancelFuncs)
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
		runner.state = stateSpawning
		runner.client = newClient("localhost", 5557, runner.nodeID)
		defer runner.shutdown()

		runner.startSpawning(10, float64(10), runner.spawnComplete)
		// wait for spawning goroutines
		time.Sleep(2 * time.Second)
		Expect(runner.numClients).To(BeEquivalentTo(10))

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

		Expect(workers).To(BeEquivalentTo(20))
		Expect(spawnRate).To(BeEquivalentTo(20))
		runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
	})

	It("test onQuitMessage", func() {
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

		runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
		Eventually(quitMessages).Should(Receive())

		runner.state = stateRunning
		runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
		Eventually(quitMessages).Should(Receive())
		Expect(runner.state).Should(BeIdenticalTo(stateInit))

		runner.state = stateStopped
		runner.onMessage(newGenericMessage("quit", nil, runner.nodeID))
		Eventually(quitMessages).Should(Receive())
		Expect(runner.state).Should(BeIdenticalTo(stateInit))
	})

	It("test on ack message", func() {
		eventCount := 0
		Events.Subscribe(EVENT_CONNECTED, func() {
			eventCount++
		})
		runner := newSlaveRunner("localhost", 5557, []*Task{}, nil)
		runner.waitForAck = sync.WaitGroup{}
		runner.waitForAck.Add(1)

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
		Expect(m.Type).To(Equal("spawning"))

		time.Sleep(2 * time.Second)
		Expect(runner.state).Should(BeIdenticalTo(stateRunning))
		Expect(runner.numClients).Should(BeEquivalentTo(10))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))

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
		Expect(runner.state).Should(BeIdenticalTo(stateRunning))
		Expect(runner.numClients).Should(BeEquivalentTo(20))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))

		// stop all the workers
		runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
		Expect(runner.state).To(BeIdenticalTo(stateInit))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("client_stopped"))

		msg = <-runner.client.sendChannel()
		crm := msg.(*clientReadyMessage)
		Expect(crm.Type).To(Equal("client_ready"))

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
		Expect(runner.state).Should(BeIdenticalTo(stateRunning))
		Expect(runner.numClients).Should(BeEquivalentTo(10))

		msg = <-runner.client.sendChannel()
		m = msg.(*genericMessage)
		Expect(m.Type).To(Equal("spawning_complete"))
		// stop all the workers
		runner.onMessage(newGenericMessage("stop", nil, runner.nodeID))
		Expect(runner.state).To(BeIdenticalTo(stateInit))

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
		Eventually(MockGomqDealerInstance.SendChannel()).Should(Receive(Equal(clientReadyMessageInBytes)))

		ackMessage := newGenericMessage("ack", nil, r.nodeID)
		ackMessageInBytes, _ := ackMessage.serialize()
		ackZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{ackMessageInBytes},
		}
		MockGomqDealerInstance.RecvChannel() <- ackZmtpMessage
	})
})
