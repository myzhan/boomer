package boomer

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync/atomic"
	"time"
)

const (
	stateInit     = "ready"
	stateHatching = "hatching"
	stateRunning  = "running"
	stateStopped  = "stopped"
	stateQuitting = "quitting"
)

const (
	slaveReportInterval = 3 * time.Second
)

// Task is like locust's task.
// when boomer receive start message, it will spawn several goroutines to run Task.Fn.
type Task struct {
	Weight int
	Fn     func()
	Name   string
}

type runner struct {
	tasks            []*Task
	numClients       int32
	hatchRate        int
	stopChannel      chan bool
	shutdownSignal   chan bool
	state            string
	masterHost       string
	masterPort       int
	client           client
	nodeID           string
	hatchType        string
	rateLimiter      rateLimiter
	rateLimitEnabled bool
	stats            *requestStats
	// cache of current time in second
	now int64
}

func newRunner(tasks []*Task, rateLimiter rateLimiter, hatchType string) (r *runner) {
	r = &runner{
		tasks:     tasks,
		hatchType: hatchType,
	}
	r.nodeID = getNodeID()
	r.shutdownSignal = make(chan bool)

	if rateLimiter != nil {
		r.rateLimitEnabled = true
		r.rateLimiter = rateLimiter
	}

	if hatchType != "asap" && hatchType != "smooth" {
		log.Fatalf("Wrong hatch-type, expected asap or smooth, was %s\n", hatchType)
	}

	r.stats = newRequestStats()

	return r
}

func (r *runner) safeRun(fn func()) {
	defer func() {
		// don't panic
		err := recover()
		if err != nil {
			stackTrace := debug.Stack()
			errMsg := fmt.Sprintf("%v", err)
			os.Stderr.Write([]byte(errMsg))
			os.Stderr.Write([]byte("\n"))
			os.Stderr.Write(stackTrace)
			RecordFailure("unknown", "panic", int64(0), errMsg)
		}
	}()
	fn()
}

func (r *runner) spawnGoRoutines(spawnCount int, quit chan bool) {
	log.Println("Hatching and swarming", spawnCount, "clients at the rate", r.hatchRate, "clients/s...")

	weightSum := 0
	for _, task := range r.tasks {
		weightSum += task.Weight
	}

	for _, task := range r.tasks {
		percent := float64(task.Weight) / float64(weightSum)
		amount := int(round(float64(spawnCount)*percent, .5, 0))

		if weightSum == 0 {
			amount = int(float64(spawnCount) / float64(len(r.tasks)))
		}

		for i := 1; i <= amount; i++ {
			if r.hatchType == "smooth" {
				time.Sleep(time.Duration(1000000/r.hatchRate) * time.Microsecond)
			} else if i%r.hatchRate == 0 {
				time.Sleep(1 * time.Second)
			}

			select {
			case <-quit:
				// quit hatching goroutine
				return
			default:
				atomic.AddInt32(&r.numClients, 1)
				go func(fn func()) {
					for {
						select {
						case <-quit:
							return
						default:
							if r.rateLimitEnabled {
								blocked := r.rateLimiter.acquire()
								if !blocked {
									r.safeRun(fn)
								}
							} else {
								r.safeRun(fn)
							}
						}
					}
				}(task.Fn)
			}
		}
	}

	r.hatchComplete()
}

func (r *runner) startHatching(spawnCount int, hatchRate int) {
	r.stats.clearStatsChannel <- true
	r.stopChannel = make(chan bool)

	r.hatchRate = hatchRate
	r.numClients = 0
	go r.spawnGoRoutines(spawnCount, r.stopChannel)
}

func (r *runner) hatchComplete() {
	data := make(map[string]interface{})
	data["count"] = r.numClients
	r.client.sendChannel() <- newMessage("hatch_complete", data, r.nodeID)
	r.state = stateRunning
}

func (r *runner) onQuiting() {
	if r.state != stateQuitting {
		r.client.sendChannel() <- newMessage("quit", nil, r.nodeID)
	}
}

func (r *runner) stop() {
	// publish the boomer stop event
	// user's code can subscribe to this event and do thins like cleaning up
	Events.Publish("boomer:stop")

	// stop previous goroutines without blocking
	// those goroutines will exit when r.safeRun returns
	close(r.stopChannel)
	if r.rateLimitEnabled {
		r.rateLimiter.stop()
	}
}

func (r *runner) close() {
	if r.client != nil {
		r.client.close()
	}
	if r.stats != nil {
		r.stats.close()
	}
	close(r.shutdownSignal)
}

func (r *runner) onHatchMessage(msg *message) {
	r.client.sendChannel() <- newMessage("hatching", nil, r.nodeID)
	rate, _ := msg.Data["hatch_rate"]
	clients, _ := msg.Data["num_clients"]
	hatchRate := int(rate.(float64))
	workers := 0
	if _, ok := clients.(uint64); ok {
		workers = int(clients.(uint64))
	} else {
		workers = int(clients.(int64))
	}
	if workers == 0 || hatchRate == 0 {
		log.Printf("Invalid hatch message from master, num_clients is %d, hatch_rate is %d\n",
			workers, hatchRate)
	} else {
		Events.Publish("boomer:hatch", workers, hatchRate)

		if r.rateLimitEnabled {
			r.rateLimiter.start()
		}
		r.startHatching(workers, hatchRate)
	}
}

// Runner acts as a state machine, and runs in one goroutine without any lock.
func (r *runner) onMessage(msg *message) {
	if msg.Type == "quit" {
		log.Println("Got quit message from master, shutting down...")
		r.state = stateQuitting
		Events.Publish("boomer:quit")
		os.Exit(0)
	}

	switch r.state {
	case stateInit:
		if msg.Type == "hatch" {
			r.state = stateHatching
			r.onHatchMessage(msg)
		}
	case stateHatching:
		fallthrough
	case stateRunning:
		switch msg.Type {
		case "hatch":
			r.state = stateHatching
			r.stop()
			r.onHatchMessage(msg)
			r.state = stateRunning
		case "stop":
			r.stop()
			r.state = stateStopped
			log.Println("Recv stop message from master, all the goroutines are stopped")
			r.client.sendChannel() <- newMessage("client_stopped", nil, r.nodeID)
			r.client.sendChannel() <- newMessage("client_ready", nil, r.nodeID)
		}
	case stateStopped:
		if msg.Type == "hatch" {
			r.state = stateHatching
			r.onHatchMessage(msg)
		}
	}
}

func (r *runner) startListener() {
	go func() {
		for {
			select {
			case msg := <-r.client.recvChannel():
				r.onMessage(msg)
			case <-r.shutdownSignal:
				return
			}
		}
	}()
}

func (r *runner) getReady() {
	r.state = stateInit
	r.client = newClient(r.masterHost, r.masterPort)
	r.client.connect()

	// listen to master
	r.startListener()

	r.stats.start()

	// tell master, I'm ready
	r.client.sendChannel() <- newMessage("client_ready", nil, r.nodeID)

	// report to master
	go func() {
		for {
			select {
			case data := <-r.stats.messageToRunner:
				if r.state == stateInit || r.state == stateStopped {
					continue
				}
				data["user_count"] = r.numClients
				r.client.sendChannel() <- newMessage("stats", data, r.nodeID)
			case <-r.shutdownSignal:
				return
			}
		}
	}()

	go func() {
		var ticker = time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				r.now = time.Now().Unix()
			case <-r.shutdownSignal:
				return
			}
		}
	}()

	Events.Subscribe("boomer:quit", r.onQuiting)
}
