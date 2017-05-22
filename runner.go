package boomer

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"
)

const (
	STATE_INIT     string = "ready"
	STATE_HATCHING string = "hatching"
	STATE_RUNNING  string = "running"
	STATE_STOPPED  string = "stopped"
)

const (
	SLAVE_REPORT_INTERVAL time.Duration = 3 * time.Second
)

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
	nodeId      string
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

	if r.state == STATE_INIT || r.state == STATE_STOPPED {
		r.state = STATE_HATCHING
		r.numClients = spawnCount
	} else {
		r.numClients += spawnCount
	}

	log.Println("Hatching and swarming", spawnCount, "clients at the rate", r.hatchRate, "clients/s...")

	weightSum := 0
	for _, task := range r.tasks {
		weightSum += task.Weight
	}

	for _, task := range r.tasks {

		percent := float64(task.Weight) / float64(weightSum)
		amount := int(Round(float64(spawnCount)*percent, .5, 0))

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
						r.safeRun(fn)
					}
				}
			}(task.Fn)
		}

	}

	r.hatchComplete()

	r.state = STATE_RUNNING

}

func (r *runner) startHatching(spawnCount int, hatchRate int) {

	if r.state != STATE_RUNNING && r.state != STATE_HATCHING {
		clearStatsChannel <- true
		r.stopChannel = make(chan bool)
		r.numClients = spawnCount
	}

	if r.state != STATE_INIT && r.state != STATE_STOPPED {
		// Dynamically changing the goroutine count
		r.state = STATE_HATCHING
		if r.numClients > spawnCount {
			// FIXME: Randomly stop goroutine, without considering their weights
			stopCount := r.numClients - spawnCount
			r.numClients -= stopCount
			for i := 0; i < stopCount; i++ {
				r.stopChannel <- true
			}
			r.hatchComplete()
		} else if r.numClients < spawnCount {
			addCount := spawnCount - r.numClients
			r.hatchRate = hatchRate
			r.spawnGoRoutines(addCount, r.stopChannel)
		} else {
			// equal
			r.hatchComplete()
		}
	} else {
		r.hatchRate = hatchRate
		r.spawnGoRoutines(spawnCount, r.stopChannel)
	}
}

func (r *runner) hatchComplete() {
	data := make(map[string]interface{})
	data["count"] = r.numClients
	toServer <- &message{
		Type:   "hatch_complete",
		Data:   data,
		NodeId: r.nodeId,
	}
}

func (r *runner) onQuiting() {
	toServer <- &message{Type: "quit", NodeId: r.nodeId}
}

func (r *runner) stop() {

	if r.state == STATE_RUNNING {
		for i := 0; i < r.numClients; i++ {
			r.stopChannel <- false
		}
		close(r.stopChannel)
		r.state = STATE_STOPPED
		log.Println("Recv stop message from master, all the goroutines are stopped")
	}

}

func (r *runner) getReady() {

	r.state = STATE_INIT

	// read message from server
	go func() {
		for {
			msg := <-fromServer
			switch msg.Type {
			case "hatch":
				toServer <- &message{Type: "hatching", NodeId: r.nodeId}
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
				toServer <- &message{Type: "client_stopped", NodeId: r.nodeId}
				toServer <- &message{Type: "client_ready", NodeId: r.nodeId}
			case "quit":
				log.Println("Got quit message from master, shutting down...")
				os.Exit(0)
			}
		}
	}()

	// tell master, I'm ready
	toServer <- &message{Type: "client_ready", NodeId: r.nodeId}

	// report to server
	go func() {
		for {
			select {
			case data := <-messageToServerChannel:
				data["user_count"] = r.numClients
				toServer <- &message{
					Type:   "stats",
					Data:   data,
					NodeId: r.nodeId,
				}
			}
		}
	}()
}
