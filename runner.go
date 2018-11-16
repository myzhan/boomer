package boomer

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	stateInit     = "ready"
	stateHatching = "hatching"
	stateRunning  = "running"
	stateStopped  = "stopped"
)

const (
	slaveReportInterval = 3 * time.Second
)

// Task is like locust's task.
// when boomer receive start message, it will spawn several
// goroutines to run Task.Fn .
type Task struct {
	Weight int
	Fn     func()
	Name   string
}

type runner struct {
	tasks          []*Task
	numClients     int32
	hatchRate      int
	stopChannel    chan bool
	shutdownSignal chan bool
	state          string
	client         client
	nodeID         string

	// RPS Control
	maxRPS                   int64
	requestIncreaseRate      string
	requestIncreaseStep      int64
	requestIncreaseInterval  time.Duration
	currentRPSThreshold      int64
	rpsThreshold             int64
	rpsControlEnabled        bool
	rpsControlChannel        chan bool
	rpsControllerQuitChannel chan bool
}

func newRunner(tasks []*Task, maxRPS int64, requestIncreaseRate string) (r *runner) {
	r = &runner{
		tasks:               tasks,
		maxRPS:              maxRPS,
		requestIncreaseRate: requestIncreaseRate,
	}
	r.nodeID = getNodeID()
	r.shutdownSignal = make(chan bool)

	if 0 < maxRPS || requestIncreaseRate != "-1" {
		r.parseRPSControlArgs()
	}
	return r
}

func (r *runner) parseRPSControlArgs() {
	r.rpsControlEnabled = true
	r.rpsControlChannel = make(chan bool)
	r.rpsControllerQuitChannel = make(chan bool)
	if r.maxRPS > 0 {
		log.Println("Max RPS that boomer may generate is limited to", r.maxRPS)
	}
	if r.requestIncreaseRate != "-1" {
		log.Println("Request increase rate is", r.requestIncreaseRate)
		if strings.Contains(r.requestIncreaseRate, "/") {
			tmp := strings.Split(r.requestIncreaseRate, "/")
			if len(tmp) != 2 {
				log.Fatalf("Wrong format of requestIncreaseRate, %s", r.requestIncreaseRate)
			}
			step, err := strconv.ParseInt(tmp[0], 10, 64)
			if err != nil {
				log.Fatalf("Failed to parse requestIncreaseRate, %v", err)
			}
			r.requestIncreaseStep = step
			r.requestIncreaseInterval, err = time.ParseDuration(tmp[1])
			if err != nil {
				log.Fatalf("Failed to parse requestIncreaseRate, %v", err)
			}
		} else {
			step, err := strconv.ParseInt(r.requestIncreaseRate, 10, 64)
			if err != nil {
				log.Fatalf("Failed to parse requestIncreaseRate, %v", err)
			}
			r.requestIncreaseStep = step
			r.requestIncreaseInterval = time.Second
		}
	}
}

func (r *runner) safeRun(fn func()) {
	defer func() {
		// don't panic
		err := recover()
		if err != nil {
			debug.PrintStack()
			Events.Publish("request_failure", "unknown", "panic", 0.0, fmt.Sprintf("%v", err))
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
			select {
			case <-quit:
				// quit hatching goroutine
				return
			default:
				if i%r.hatchRate == 0 {
					time.Sleep(1 * time.Second)
				}
				atomic.AddInt32(&r.numClients, 1)
				go func(fn func()) {
					for {
						select {
						case <-quit:
							return
						default:
							if r.rpsControlEnabled {
								token := atomic.AddInt64(&r.rpsThreshold, -1)
								if token < 0 {
									// RPS threshold is reached, wait until next second
									<-r.rpsControlChannel
								} else {
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
	defaultStats.clearStatsChannel <- true
	r.stopChannel = make(chan bool)

	r.hatchRate = hatchRate
	r.numClients = 0
	go r.spawnGoRoutines(spawnCount, r.stopChannel)
}

func (r *runner) hatchComplete() {
	data := make(map[string]interface{})
	data["count"] = r.numClients
	toMaster <- newMessage("hatch_complete", data, r.nodeID)
}

func (r *runner) onQuiting() {
	toMaster <- newMessage("quit", nil, r.nodeID)
}

func (r *runner) stop() {
	// stop previous goroutines without blocking
	// those goroutines will exit when r.safeRun returns
	close(r.stopChannel)
	if r.rpsControlEnabled {
		r.stopRPSController()
	}
}

func (r *runner) close() {
	close(r.shutdownSignal)
}

func (r *runner) onHatchMessage(msg *message, rpsControllerStarted bool) {
	toMaster <- newMessage("hatching", nil, r.nodeID)
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
		if r.rpsControlEnabled && !rpsControllerStarted {
			r.startRPSController()
		}
		r.startHatching(workers, hatchRate)
	}
}

// Runner acts as a state machine, and runs in one goroutine without any lock.
func (r *runner) onMessage(msg *message) {
	if msg.Type == "quit" {
		log.Println("Got quit message from master, shutting down...")
		os.Exit(0)
	}

	switch r.state {
	case stateInit:
		if msg.Type == "hatch" {
			r.state = stateHatching
			r.onHatchMessage(msg, false)
			r.state = stateRunning
		}
	case stateHatching:
		fallthrough
	case stateRunning:
		switch msg.Type {
		case "hatch":
			r.state = stateHatching
			r.onHatchMessage(msg, true)
			r.state = stateRunning
		case "stop":
			r.stop()
			r.state = stateStopped
			log.Println("Recv stop message from master, all the goroutines are stopped")
			toMaster <- newMessage("client_stopped", nil, r.nodeID)
			toMaster <- newMessage("client_ready", nil, r.nodeID)
		}
	case stateStopped:
		if msg.Type == "hatch" {
			r.state = stateHatching
			r.onHatchMessage(msg, false)
			r.state = stateRunning
		}
	}
}

func (r *runner) startListener() {
	go func() {
		for {
			select {
			case msg := <-fromMaster:
				r.onMessage(msg)
			case <-r.shutdownSignal:
				return
			}
		}
	}()
}

func (r *runner) startBucketUpdater() {
	go func() {
		for {
			select {
			case <-r.shutdownSignal:
				return
			case <-r.rpsControllerQuitChannel:
				return
			default:
				atomic.StoreInt64(&r.rpsThreshold, r.currentRPSThreshold)
				time.Sleep(1 * time.Second)
				// use channel to broadcast
				close(r.rpsControlChannel)
				r.rpsControlChannel = make(chan bool)
			}
		}
	}()
}

func (r *runner) startRPSController() {
	r.rpsControllerQuitChannel = make(chan bool)
	go func() {
		for {
			select {
			case <-r.shutdownSignal:
				return
			case <-r.rpsControllerQuitChannel:
				return
			default:
				if r.requestIncreaseStep > 0 {
					r.currentRPSThreshold = r.currentRPSThreshold + r.requestIncreaseStep
					if r.currentRPSThreshold < 0 {
						// int64 overflow
						r.currentRPSThreshold = int64(math.MaxInt64)
					}
					if r.maxRPS > 0 && r.currentRPSThreshold > r.maxRPS {
						r.currentRPSThreshold = r.maxRPS
					}
				} else {
					if r.maxRPS > 0 {
						r.currentRPSThreshold = r.maxRPS
					}
				}
				time.Sleep(r.requestIncreaseInterval)
			}
		}
	}()
	r.startBucketUpdater()
}

func (r *runner) stopRPSController() {
	close(r.rpsControllerQuitChannel)
}

func (r *runner) getReady() {
	r.state = stateInit
	r.client = newClient()

	// listen to master
	r.startListener()

	// tell master, I'm ready
	toMaster <- newMessage("client_ready", nil, r.nodeID)

	// report to master
	go func() {
		for {
			select {
			case data := <-defaultStats.messageToRunner:
				data["user_count"] = r.numClients
				toMaster <- newMessage("stats", data, r.nodeID)
			case <-r.shutdownSignal:
				return
			}
		}
	}()

	Events.Subscribe("boomer:quit", r.onQuiting)
}
