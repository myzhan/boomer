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
	tasks       []*Task
	numClients  int
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
			if i%r.hatchRate == 0 {
				time.Sleep(1 * time.Second)
			}
			go func(fn func()) {
				for {
					select {
					case <-quit:
						return
					default:
						if maxRPSEnabled {
							token := atomic.AddInt64(&maxRPSThreshold, -1)
							if token < 0 {
								// max RPS is reached, wait until next second
								<-maxRPSControlChannel
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

	r.hatchComplete()

}

func (r *runner) startHatching(spawnCount int, hatchRate int) {

	if r.state != stateRunning && r.state != stateHatching {
		clearStatsChannel <- true
		r.stopChannel = make(chan bool)
	}

	if r.state == stateRunning {
		// stop previous goroutines without blocking
		// those goroutines will exit when r.safeRun returns
		close(r.stopChannel)
	}

	r.stopChannel = make(chan bool)
	r.state = stateHatching

	r.hatchRate = hatchRate
	r.numClients = spawnCount
	r.spawnGoRoutines(r.numClients, r.stopChannel)

}

func (r *runner) hatchComplete() {

	data := make(map[string]interface{})
	data["count"] = r.numClients
	toServer <- newMessage("hatch_complete", data, r.nodeID)

	r.state = stateRunning
}

func (r *runner) onQuiting() {
	toServer <- newMessage("quit", nil, r.nodeID)
}

func (r *runner) stop() {

	if r.state == stateRunning {
		close(r.stopChannel)
		r.state = stateStopped
		log.Println("Recv stop message from master, all the goroutines are stopped")
	}

}

func (r *runner) getReady() {

	r.state = stateInit

	// read message from server
	go func() {
		for {
			msg := <-fromServer
			switch msg.Type {
			case "hatch":
				toServer <- newMessage("hatching", nil, r.nodeID)
				rate, _ := msg.Data["hatch_rate"]
				clients, _ := msg.Data["num_clients"]
				hatchRate := rate.(float64)
				workers := 0
				if _, ok := clients.(uint64); ok {
					workers = int(clients.(uint64))
				} else {
					workers = int(clients.(int64))
				}
				r.startHatching(workers, int(hatchRate))
			case "stop":
				r.stop()
				toServer <- newMessage("client_stopped", nil, r.nodeID)
				toServer <- newMessage("client_ready", nil, r.nodeID)
			case "quit":
				log.Println("Got quit message from master, shutting down...")
				os.Exit(0)
			}
		}
	}()

	// tell master, I'm ready
	toServer <- newMessage("client_ready", nil, r.nodeID)

	// report to server
	go func() {
		for {
			select {
			case data := <-messageToServerChannel:
				data["user_count"] = r.numClients
				toServer <- newMessage("stats", data, r.nodeID)
			}
		}
	}()

	if maxRPSEnabled {
		go func() {
			for {
				atomic.StoreInt64(&maxRPSThreshold, maxRPS)
				time.Sleep(1 * time.Second)
				// use channel to broadcast
				close(maxRPSControlChannel)
				maxRPSControlChannel = make(chan bool)
			}
		}()
	}
}
