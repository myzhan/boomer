package boomer

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
	"sync"
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
	heartbeatInterval   = 1 * time.Second
)

type runner struct {
	state string

	tasks           []*Task
	totalTaskWeight int

	rateLimiter      RateLimiter
	rateLimitEnabled bool
	stats            *requestStats

	numClients int32
	hatchRate  float64
	host       string

	// all running workers(goroutines) will select on this channel.
	// close this channel will stop all running workers.
	stopChan chan bool

	// close this channel will stop all goroutines used in runner.
	closeChan chan bool

	outputs []Output
}

// safeRun runs fn and recovers from unexpected panics.
// it prevents panics from Task.Fn crashing boomer.
func (r *runner) safeRun(args TaskArgs, fn func(args TaskArgs)) {
	defer func() {
		// don't panic
		err := recover()
		if err != nil {
			stackTrace := debug.Stack()
			errMsg := fmt.Sprintf("%v", err)
			os.Stderr.Write([]byte(errMsg))
			os.Stderr.Write([]byte("\n"))
			os.Stderr.Write(stackTrace)
		}
	}()
	fn(args)
}

func (r *runner) addOutput(o Output) {
	r.outputs = append(r.outputs, o)
}

func (r *runner) outputOnStart() {
	size := len(r.outputs)
	if size == 0 {
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(size)
	for _, output := range r.outputs {
		go func(o Output) {
			o.OnStart()
			wg.Done()
		}(output)
	}
	wg.Wait()
}

func (r *runner) outputOnEevent(data map[string]interface{}) {
	size := len(r.outputs)
	if size == 0 {
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(size)
	for _, output := range r.outputs {
		go func(o Output) {
			o.OnEvent(data)
			wg.Done()
		}(output)
	}
	wg.Wait()
}

func (r *runner) outputOnStop() {
	size := len(r.outputs)
	if size == 0 {
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(size)
	for _, output := range r.outputs {
		go func(o Output) {
			o.OnStop()
			wg.Done()
		}(output)
	}
	wg.Wait()
}

func (r *runner) spawnWorkers(args TaskArgs, spawnCount int, quit chan bool, hatchCompleteFunc func()) {
	log.Printf("Hatching and swarming %d clients at the rate %.2f clients/s, targeting %q", spawnCount, r.hatchRate, args.Host)

	defer func() {
		if hatchCompleteFunc != nil {
			hatchCompleteFunc()
		}
	}()

	for i := 1; i <= spawnCount; i++ {
		sleepTime := time.Duration(1000000/r.hatchRate) * time.Microsecond
		time.Sleep(sleepTime)

		select {
		case <-quit:
			// quit hatching goroutine
			return
		default:
			atomic.AddInt32(&r.numClients, 1)
			go func() {
				for {
					task := r.getTask()
					select {
					case <-quit:
						return
					default:
						if r.rateLimitEnabled {
							blocked := r.rateLimiter.Acquire()
							if !blocked {
								r.safeRun(TaskArgs{Host: r.host}, task.Fn)
							}
						} else {
							r.safeRun(args, task.Fn)
						}
					}
				}
			}()
		}
	}
}

// setTasks will set the runner's task list AND the total task weight
// which is used to get a random task later
func (r *runner) setTasks(t []*Task) {
	r.tasks = t

	weightSum := 0
	for _, task := range r.tasks {
		weightSum += task.Weight
	}
	r.totalTaskWeight = weightSum
}

func (r *runner) getTask() *Task {
	tasksCount := len(r.tasks)
	if tasksCount == 1 {
		// Fast path
		return r.tasks[0]
	}

	rs := rand.New(rand.NewSource(time.Now().UnixNano()))

	totalWeight := r.totalTaskWeight
	if totalWeight <= 0 {
		// If all the tasks have not weights defined, they have the same chance to run
		randNum := rs.Intn(tasksCount)
		return r.tasks[randNum]
	}

	randNum := rs.Intn(totalWeight)
	runningSum := 0
	for _, task := range r.tasks {
		runningSum += task.Weight
		if runningSum > randNum {
			return task
		}
	}

	return nil
}

func (r *runner) startHatching(args TaskArgs, spawnCount int, hatchRate float64, hatchCompleteFunc func()) {
	r.stats.clearStatsChan <- true
	r.stopChan = make(chan bool)

	r.hatchRate = hatchRate
	r.numClients = 0

	go r.spawnWorkers(args, spawnCount, r.stopChan, hatchCompleteFunc)
}

func (r *runner) stop() {
	// publish the boomer stop event
	// user's code can subscribe to this event and do thins like cleaning up
	Events.Publish("boomer:stop")

	// stop previous goroutines without blocking
	// those goroutines will exit when r.safeRun returns
	close(r.stopChan)
	if r.rateLimitEnabled {
		r.rateLimiter.Stop()
	}
}

type localRunner struct {
	runner

	hatchCount int
}

func newLocalRunner(tasks []*Task, rateLimiter RateLimiter, hatchCount int, hatchRate float64) (r *localRunner) {
	r = &localRunner{}
	r.setTasks(tasks)
	r.hatchRate = hatchRate
	r.hatchCount = hatchCount
	r.closeChan = make(chan bool)
	r.addOutput(NewConsoleOutput())
	r.host = host

	if rateLimiter != nil {
		r.rateLimitEnabled = true
		r.rateLimiter = rateLimiter
	}

	r.stats = newRequestStats()
	return r
}

func (r *localRunner) run() {
	r.state = stateInit
	r.stats.start()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for {
			select {
			case data := <-r.stats.messageToRunnerChan:
				data["user_count"] = r.numClients
				r.outputOnEevent(data)
			case <-r.closeChan:
				Events.Publish("boomer:quit")
				r.stop()
				wg.Done()
				return
			}
		}
	}()

	if r.rateLimitEnabled {
		r.rateLimiter.Start()
	}
	r.startHatching(TaskArgs{Host: r.host}, r.hatchCount, r.hatchRate, nil)

	wg.Wait()
}

func (r *localRunner) close() {
	if r.stats != nil {
		r.stats.close()
	}
	close(r.closeChan)
}

// SlaveRunner connects to the master, spawns goroutines and collects stats.
type slaveRunner struct {
	runner

	nodeID     string
	masterHost string
	masterPort int
	client     client
}

func newSlaveRunner(masterHost string, masterPort int, tasks []*Task, rateLimiter RateLimiter) (r *slaveRunner) {
	r = &slaveRunner{}
	r.masterHost = masterHost
	r.masterPort = masterPort
	r.setTasks(tasks)
	r.nodeID = getNodeID()
	r.closeChan = make(chan bool)

	if rateLimiter != nil {
		r.rateLimitEnabled = true
		r.rateLimiter = rateLimiter
	}

	r.stats = newRequestStats()
	return r
}

func (r *slaveRunner) hatchComplete() {
	data := make(map[string]interface{})
	data["count"] = r.numClients
	r.client.sendChannel() <- newMessage("hatch_complete", data, r.nodeID)
	r.state = stateRunning
}

func (r *slaveRunner) onQuiting() {
	if r.state != stateQuitting {
		r.client.sendChannel() <- newMessage("quit", nil, r.nodeID)
	}
}

func (r *slaveRunner) close() {
	if r.stats != nil {
		r.stats.close()
	}
	if r.client != nil {
		r.client.close()
	}
	close(r.closeChan)
}

func (r *slaveRunner) onHatchMessage(msg *message) {
	log.Printf("msg.Data: %#v", msg.Data)
	r.client.sendChannel() <- newMessage("hatching", nil, r.nodeID)
	var target = host
	if ti, ok := msg.Data["host"]; ok {
		switch t := ti.(type) {
		case string:
			target = t
		case []uint8:
			bs := make([]byte, len(t))
			for i := range t {
				bs[i] = t[i]
			}
			target = string(bs)
		}
	}

	rate, _ := msg.Data["hatch_rate"]
	users, ok := msg.Data["num_users"]
	if !ok {
		// keep compatible with previous version of locust.
		users, _ = msg.Data["num_clients"]
	}
	hatchRate := rate.(float64)
	workers := 0
	if _, ok := users.(uint64); ok {
		workers = int(users.(uint64))
	} else {
		workers = int(users.(int64))
	}

	Events.Publish("boomer:hatch", workers, hatchRate)

	if r.rateLimitEnabled {
		r.rateLimiter.Start()
	}
	r.startHatching(TaskArgs{Host: target}, workers, hatchRate, r.hatchComplete)
}

// Runner acts as a state machine.
func (r *slaveRunner) onMessage(msg *message) {
	switch r.state {
	case stateInit:
		switch msg.Type {
		case "hatch":
			r.state = stateHatching
			r.onHatchMessage(msg)
		case "quit":
			Events.Publish("boomer:quit")
		}
	case stateHatching:
		fallthrough
	case stateRunning:
		switch msg.Type {
		case "hatch":
			r.state = stateHatching
			r.stop()
			r.onHatchMessage(msg)
		case "stop":
			r.stop()
			r.state = stateStopped
			log.Println("Recv stop message from master, all the goroutines are stopped")
			r.client.sendChannel() <- newMessage("client_stopped", nil, r.nodeID)
			r.client.sendChannel() <- newMessage("client_ready", nil, r.nodeID)
			r.state = stateInit
		case "quit":
			r.stop()
			log.Println("Recv quit message from master, all the goroutines are stopped")
			Events.Publish("boomer:quit")
			r.state = stateInit
		}
	case stateStopped:
		switch msg.Type {
		case "hatch":
			r.state = stateHatching
			r.onHatchMessage(msg)
		case "quit":
			Events.Publish("boomer:quit")
			r.state = stateInit
		}
	}
}

func (r *slaveRunner) startListener() {
	go func() {
		for {
			select {
			case msg := <-r.client.recvChannel():
				r.onMessage(msg)
			case <-r.closeChan:
				return
			}
		}
	}()
}

func (r *slaveRunner) run() {
	r.state = stateInit
	r.client = newClient(r.masterHost, r.masterPort, r.nodeID)

	err := r.client.connect()
	if err != nil {
		if strings.Contains(err.Error(), "Socket type DEALER is not compatible with PULL") {
			log.Println("Newer version of locust changes ZMQ socket to DEALER and ROUTER, you should update your locust version.")
		} else {
			log.Printf("Failed to connect to master(%s:%d) with error %v\n", r.masterHost, r.masterPort, err)
		}
		return
	}

	// listen to master
	r.startListener()

	r.stats.start()

	// tell master, I'm ready
	r.client.sendChannel() <- newMessage("client_ready", nil, r.nodeID)

	// report to master
	go func() {
		for {
			select {
			case data := <-r.stats.messageToRunnerChan:
				if r.state == stateInit || r.state == stateStopped {
					continue
				}
				data["user_count"] = r.numClients
				r.client.sendChannel() <- newMessage("stats", data, r.nodeID)
				r.outputOnEevent(data)
			case <-r.closeChan:
				return
			}
		}
	}()

	// heartbeat
	// See: https://github.com/locustio/locust/commit/a8c0d7d8c588f3980303358298870f2ea394ab93
	go func() {
		var ticker = time.NewTicker(heartbeatInterval)
		for {
			select {
			case <-ticker.C:
				CPUUsage := GetCurrentCPUUsage()
				data := map[string]interface{}{
					"state":             r.state,
					"current_cpu_usage": CPUUsage,
				}
				r.client.sendChannel() <- newMessage("heartbeat", data, r.nodeID)
			case <-r.closeChan:
				return
			}
		}
	}()

	Events.Subscribe("boomer:quit", r.onQuiting)
}
