package boomer

import (
	"fmt"
	"log"
	"math"
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

type TaskClient interface {
	New() TaskClient
	OnStart(idx int) bool
	Tasks() []*Task
	OnStop()
}

type runner struct {
	tasks       []*Task
	taskClients TaskClient
	numClients  int32
	hatchRate   int
	stopChannel chan bool
	state       string
	client      client
	nodeID      string
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
	if len(r.taskClients.Tasks()) > 0 {
		r.runTaskClients(spawnCount, quit)
	} else {
		r.runTasks(spawnCount, quit)
	}
	r.hatchComplete()

}

func (r *runner) runTaskClients(spawnCount int, quit chan bool) {

	weightSum := 0
	tasks := r.taskClients.Tasks()
	for _, task := range tasks {
		weightSum += task.Weight
	}

	idx := 0
	for task_idx, task := range tasks {

		percent := float64(task.Weight) / float64(weightSum)
		amount := int(round(float64(spawnCount)*percent, .5, 0))

		if weightSum == 0 {
			amount = int(float64(spawnCount) / float64(len(tasks)))
		}

		for i := 1; i <= amount; i++ {
			select {
			case <-quit:
				// quit hatching goroutine
				return
			default:
				idx += 1
				c := r.taskClients.New()
				if !c.OnStart(idx) {
					continue
				}
				if i%r.hatchRate == 0 {
					time.Sleep(1 * time.Second)
				}
				atomic.AddInt32(&r.numClients, 1)
				go func(c TaskClient, task_idx int) {
					cts := c.Tasks()
					fn := cts[task_idx].Fn
					for {
						select {
						case <-quit:
							c.OnStop()
							return
						default:
							if rpsControlEnabled {
								token := atomic.AddInt64(&rpsThreshold, -1)
								if token < 0 {
									// RPS threshold is reached, wait until next second
									<-rpsControlChannel
								} else {
									r.safeRun(fn)
								}
							} else {
								r.safeRun(fn)
							}
						}
					}
				}(c, task_idx)
			}
		}
	}
}

func (r *runner) runTasks(spawnCount int, quit chan bool) {
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
							if rpsControlEnabled {
								token := atomic.AddInt64(&rpsThreshold, -1)
								if token < 0 {
									// RPS threshold is reached, wait until next second
									<-rpsControlChannel
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
}
func (r *runner) startHatching(spawnCount int, hatchRate int) {

	if r.state != stateRunning && r.state != stateHatching {
		clearStatsChannel <- true
		r.stopChannel = make(chan bool)
	}

	if r.state == stateRunning || r.state == stateHatching {
		// stop previous goroutines without blocking
		// those goroutines will exit when r.safeRun returns
		close(r.stopChannel)
	}

	r.stopChannel = make(chan bool)
	r.state = stateHatching

	r.hatchRate = hatchRate
	r.numClients = 0
	go r.spawnGoRoutines(spawnCount, r.stopChannel)
}

func (r *runner) hatchComplete() {

	data := make(map[string]interface{})
	data["count"] = r.numClients
	toMaster <- newMessage("hatch_complete", data, r.nodeID)

	r.state = stateRunning
}

func (r *runner) onQuiting() {
	toMaster <- newMessage("quit", nil, r.nodeID)
}

func (r *runner) stop() {

	if r.state == stateRunning || r.state == stateHatching {
		close(r.stopChannel)
		r.state = stateStopped
		log.Println("Recv stop message from master, all the goroutines are stopped")
	}

}

func (r *runner) listener() {
	for {
		msg := <-fromMaster
		switch msg.Type {
		case "hatch":
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
				if rpsControlEnabled {
					startRPSController()
				}
				r.startHatching(workers, hatchRate)
			}
		case "stop":
			r.stop()
			if rpsControlEnabled {
				stopRPSController()
			}
			toMaster <- newMessage("client_stopped", nil, r.nodeID)
			toMaster <- newMessage("client_ready", nil, r.nodeID)
		case "quit":
			log.Println("Got quit message from master, shutting down...")
			os.Exit(0)
		}
	}
}

func startBucketUpdater() {
	go func() {
		for {
			select {
			case <-rpsControllerQuitChannel:
				return
			default:
				atomic.StoreInt64(&rpsThreshold, currentRPSThreshold)
				time.Sleep(1 * time.Second)
				// use channel to broadcast
				close(rpsControlChannel)
				rpsControlChannel = make(chan bool)
			}
		}
	}()
}

func startRPSController() {
	rpsControllerQuitChannel = make(chan bool)
	go func() {
		for {
			select {
			case <-rpsControllerQuitChannel:
				return
			default:
				if requestIncreaseStep > 0 {
					currentRPSThreshold = currentRPSThreshold + requestIncreaseStep
					if currentRPSThreshold < 0 {
						// int64 overflow
						currentRPSThreshold = int64(math.MaxInt64)
					}
					if maxRPS > 0 && currentRPSThreshold > maxRPS {
						currentRPSThreshold = maxRPS
					}
				} else {
					if maxRPS > 0 {
						currentRPSThreshold = maxRPS
					}
				}
				time.Sleep(requestIncreaseInterval)
			}
		}
	}()
	startBucketUpdater()
}

func stopRPSController() {
	close(rpsControllerQuitChannel)
}

func (r *runner) getReady() {

	r.state = stateInit

	// listen to master
	go r.listener()

	// tell master, I'm ready
	toMaster <- newMessage("client_ready", nil, r.nodeID)

	// report to master
	go func() {
		for {
			select {
			case data := <-messageToRunner:
				data["user_count"] = r.numClients
				toMaster <- newMessage("stats", data, r.nodeID)
			}
		}
	}()

}
