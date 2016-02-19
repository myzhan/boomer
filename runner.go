package boomer


import (
	"fmt"
	"log"
	"time"
	"os"
	"runtime/debug"
)


const (
	STATE_INIT string = "ready"
	STATE_HATCHING string = "hatching"
	STATE_RUNNING string = "running"
	STATE_STOPPED string = "stopped"
)


const (
	SLAVE_REPORT_INTERVAL time.Duration = 3 * time.Second
)


type Task struct {

	Weight int
	Fn     func()
	Name   string
}


type Runner struct {

	Tasks       []*Task
	NumClients  int
	hatchRate   int
	stopChannel chan bool
	state       string
	Client      Client
	NodeId      string
}


func (this *Runner) safeRun(fn func()){
	defer func(){
		// don't panic
		err := recover()
		if err != nil{
			debug.PrintStack()
			Events.Publish("request_failure", "unknown", "panic", 0.0, fmt.Sprintf("%v", err))
		}
	}()
	fn()
}


func (this *Runner) spawnGoRoutines(spawnCount int, quit chan bool) {

	if this.state == STATE_INIT || this.state == STATE_STOPPED {
		this.state = STATE_HATCHING
		this.NumClients = spawnCount
	}else {
		this.NumClients += spawnCount
	}

	log.Println("Hatching and swarming", spawnCount, "clients at the rate", this.hatchRate, "clients/s...")

	weightSum := 0
	for _, task := range this.Tasks {
		weightSum += task.Weight
	}

	for _, task := range this.Tasks {

		percent := float64(task.Weight) / float64(weightSum)
		amount := int(Round(float64(spawnCount) * percent, .5, 0))

		for i := 1; i <= amount; i++ {
			if i % this.hatchRate == 0 {
				time.Sleep(1 * time.Second)
			}
			go func(fn func()) {
				for{
					select {
					case <- quit:
						return
					default:
						this.safeRun(fn)
					}
				}
			}(task.Fn)
		}

	}

	this.hatchComplete()

	this.state = STATE_RUNNING

}


func (this *Runner) StartHatching(spawnCount int, hatchRate int) {

	if this.state != STATE_RUNNING && this.state != STATE_HATCHING {
		ClearStatsChannel <- true
		this.stopChannel = make(chan bool)
		this.NumClients = spawnCount
	}


	if this.state != STATE_INIT && this.state != STATE_STOPPED {
		// Dynamically changing the goroutine count
		this.state = STATE_HATCHING
		if this.NumClients > spawnCount {
			// FIXME: Randomly stop goroutine, without considering their weights
			stopCount := this.NumClients - spawnCount
			this.NumClients -= stopCount
			for i := 0; i < stopCount; i++ {
				this.stopChannel <- true
			}
			this.hatchComplete()
		}else if this.NumClients < spawnCount {
			addCount := spawnCount - this.NumClients
			this.hatchRate = hatchRate
			this.spawnGoRoutines(addCount, this.stopChannel)
		}else {
			// equal
			this.hatchComplete()
		}
	}else {
		this.hatchRate = hatchRate
		this.spawnGoRoutines(spawnCount, this.stopChannel)
	}
}


func (this *Runner) hatchComplete() {
	data := make(map[string]interface{})
	data["count"] = this.NumClients
	ToServer <- &Message{
		Type: "hatch_complete",
		Data: data,
		NodeId: this.NodeId,
	}
}


func (this *Runner) onReportToMaster(data *map[string]interface{}) {
	(*data)["user_count"] = this.NumClients
}


func (this *Runner) onQuiting() {
	ToServer <- &Message{Type: "quit", NodeId: this.NodeId}
}


func (this *Runner) Stop() {

	if this.state == STATE_RUNNING {
		for i := 0; i < this.NumClients; i++ {
			this.stopChannel <- false
		}
		close(this.stopChannel)
		this.state = STATE_STOPPED
		log.Println("Recv stop message from master, all the goroutines are stopped")
	}

}


func (this *Runner) GetReady() {

	this.state = STATE_INIT

	// read message from server
	go func() {
		for {
			msg := <-FromServer
			switch msg.Type {
			case "hatch":
				ToServer <- &Message{Type: "hatching", NodeId: this.NodeId}
				rate, _ := msg.Data["hatch_rate"]
				clients, _ := msg.Data["num_clients"]
				hatchRate := rate.(float64)
				workers := 0
				if _, ok := clients.(uint64); ok {
					workers = int(clients.(uint64))
				}else {
					workers = int(clients.(int64))
				}
				this.StartHatching(workers, int(hatchRate))
			case "stop":
				this.Stop()
				ToServer <- &Message{Type: "client_stopped", NodeId: this.NodeId}
				ToServer <- &Message{Type: "client_ready", NodeId: this.NodeId}
			case "quit":
				log.Println("Got quit message from master, shutting down...")
				os.Exit(0)
			}
		}
	}()

	// tell master, I'm ready
	ToServer <- &Message{Type: "client_ready", NodeId: this.NodeId}

	// report to server
	go func() {
		var ticker = time.NewTicker(SLAVE_REPORT_INTERVAL)
		for {
			select {
			case <-ticker.C:
				data := make(map[string]interface{})
			// collect stats data
				Events.Publish("boomer:report_to_master", &data)
				ToServer <- &Message{
					Type: "stats",
					Data: data,
					NodeId: this.NodeId,
				}
			}
		}
	}()
}