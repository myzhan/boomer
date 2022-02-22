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
	stateSpawning = "spawning"
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

	// TODO: we save user_class_count in spawn message and send it back to master without modification, may be a bad idea?
	userClassesCountFromMaster map[string]int64

	numClients int32
	spawnRate  float64

	// all running workers(goroutines) will select on this channel.
	// close this channel will stop all running workers.
	stopChan chan bool

	// close this channel will stop all goroutines used in runner, including running workers.
	shutdownChan chan bool

	outputs []Output
}

// safeRun runs fn and recovers from unexpected panics.
// it prevents panics from Task.Fn crashing boomer.
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
		}
	}()
	fn()
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

func (r *runner) spawnWorkers(spawnCount int, quit chan bool, spawnCompleteFunc func()) {
	log.Println("Spawning", spawnCount, "clients immediately")

	for i := 1; i <= spawnCount; i++ {
		select {
		case <-quit:
			// quit spawning goroutine
			return
		case <-r.shutdownChan:
			return
		default:
			atomic.AddInt32(&r.numClients, 1)
			go func() {
				for {
					select {
					case <-quit:
						return
					case <-r.shutdownChan:
						return
					default:
						if r.rateLimitEnabled {
							blocked := r.rateLimiter.Acquire()
							if !blocked {
								task := r.getTask()
								r.safeRun(task.Fn)
							}
						} else {
							task := r.getTask()
							r.safeRun(task.Fn)
						}
					}
				}
			}()
		}
	}

	if spawnCompleteFunc != nil {
		spawnCompleteFunc()
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

func (r *runner) startSpawning(spawnCount int, spawnRate float64, spawnCompleteFunc func()) {
	Events.Publish(EVENT_SPAWN, spawnCount, spawnRate)

	r.stopChan = make(chan bool)
	r.numClients = 0

	go r.spawnWorkers(spawnCount, r.stopChan, spawnCompleteFunc)
}

func (r *runner) stop() {
	// publish the boomer stop event
	// user's code can subscribe to this event and do thins like cleaning up
	Events.Publish(EVENT_STOP)

	// stop previous goroutines without blocking
	// those goroutines will exit when r.safeRun returns
	close(r.stopChan)
}

type localRunner struct {
	runner

	spawnCount int
}

func newLocalRunner(tasks []*Task, rateLimiter RateLimiter, spawnCount int, spawnRate float64) (r *localRunner) {
	r = &localRunner{}
	r.setTasks(tasks)
	r.spawnRate = spawnRate
	r.spawnCount = spawnCount
	r.shutdownChan = make(chan bool)

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
	r.outputOnStart()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for {
			select {
			case data := <-r.stats.messageToRunnerChan:
				data["user_count"] = r.numClients
				r.outputOnEevent(data)
			case <-r.shutdownChan:
				Events.Publish(EVENT_QUIT)
				r.stop()
				wg.Done()
				r.outputOnStop()
				return
			}
		}
	}()

	if r.rateLimitEnabled {
		r.rateLimiter.Start()
	}
	r.startSpawning(r.spawnCount, r.spawnRate, nil)

	wg.Wait()
}

func (r *localRunner) shutdown() {
	if r.stats != nil {
		r.stats.close()
	}
	if r.rateLimitEnabled {
		r.rateLimiter.Stop()
	}
	close(r.shutdownChan)
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
	r.shutdownChan = make(chan bool)

	if rateLimiter != nil {
		r.rateLimitEnabled = true
		r.rateLimiter = rateLimiter
	}

	r.stats = newRequestStats()
	return r
}

func (r *slaveRunner) spawnComplete() {
	data := make(map[string]interface{})
	data["count"] = r.numClients
	data["user_classes_count"] = r.userClassesCountFromMaster
	r.client.sendChannel() <- newGenericMessage("spawning_complete", data, r.nodeID)
	r.state = stateRunning
}

func (r *slaveRunner) onQuiting() {
	if r.state != stateQuitting {
		r.client.sendChannel() <- newGenericMessage("quit", nil, r.nodeID)
	}
}

func (r *slaveRunner) shutdown() {
	if r.stats != nil {
		r.stats.close()
	}
	if r.client != nil {
		r.client.close()
	}
	if r.rateLimitEnabled {
		r.rateLimiter.Stop()
	}
	close(r.shutdownChan)
}

func (r *slaveRunner) sumUsersAmount(msg *genericMessage) int {
	userClassesCount := msg.Data["user_classes_count"]
	userClassesCountMap := userClassesCount.(map[interface{}]interface{})

	// Save the original field and send it back to master in spawnComplete message.
	r.userClassesCountFromMaster = make(map[string]int64)
	amount := 0
	for class, num := range userClassesCountMap {
		c, ok := class.(string)
		n, ok2 := castToInt64(num)
		if !ok || !ok2 {
			log.Printf("user_classes_count in spawn message can't be casted to map[string]int64, current type is map[%T]%T, ignored!\n", class, num)
			continue
		}
		r.userClassesCountFromMaster[c] = n
		amount = amount + int(n)
	}
	return amount
}

// TODO: Since locust 2.0, spawn rate and user count are both handled by master.
// But user count is divided by user classes defined in locustfile, because locust assumes that
// master and workers use the same locustfile. Before we find a better way to deal with this,
// boomer sums up the total amout of users in spawn message and uses task weight to spawn goroutines like before.
func (r *slaveRunner) onSpawnMessage(msg *genericMessage) {
	r.client.sendChannel() <- newGenericMessage("spawning", nil, r.nodeID)
	workers := r.sumUsersAmount(msg)
	r.startSpawning(workers, float64(workers), r.spawnComplete)
}

// Runner acts as a state machine.
func (r *slaveRunner) onMessage(msgInterface message) {
	msg, ok := msgInterface.(*genericMessage)
	if !ok {
		log.Println("Receive unknown type of meesage from master.")
		return
	}

	switch r.state {
	case stateInit:
		switch msg.Type {
		case "spawn":
			r.state = stateSpawning
			r.stats.clearStatsChan <- true
			r.onSpawnMessage(msg)
		case "quit":
			Events.Publish(EVENT_QUIT)
		}
	case stateSpawning:
		fallthrough
	case stateRunning:
		switch msg.Type {
		case "spawn":
			r.state = stateSpawning
			r.stop()
			r.onSpawnMessage(msg)
		case "stop":
			r.stop()
			r.state = stateStopped
			log.Println("Recv stop message from master, all the goroutines are stopped")
			r.client.sendChannel() <- newGenericMessage("client_stopped", nil, r.nodeID)
			r.client.sendChannel() <- newClientReadyMessage("client_ready", -1, r.nodeID)
			r.state = stateInit
		case "quit":
			r.stop()
			log.Println("Recv quit message from master, all the goroutines are stopped")
			Events.Publish(EVENT_QUIT)
			r.state = stateInit
		}
	case stateStopped:
		switch msg.Type {
		case "spawn":
			r.state = stateSpawning
			r.stats.clearStatsChan <- true
			r.onSpawnMessage(msg)
		case "quit":
			Events.Publish(EVENT_QUIT)
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
			case <-r.shutdownChan:
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
	r.outputOnStart()

	if r.rateLimitEnabled {
		r.rateLimiter.Start()
	}

	// tell master, I'm ready
	// locust allows workers to bypass version check by sending -1 as version
	r.client.sendChannel() <- newClientReadyMessage("client_ready", -1, r.nodeID)

	// report to master
	go func() {
		for {
			select {
			case data := <-r.stats.messageToRunnerChan:
				if r.state == stateInit || r.state == stateStopped {
					continue
				}
				data["user_count"] = r.numClients
				data["user_classes_count"] = r.userClassesCountFromMaster
				r.client.sendChannel() <- newGenericMessage("stats", data, r.nodeID)
				r.outputOnEevent(data)
			case <-r.shutdownChan:
				r.outputOnStop()
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
				r.client.sendChannel() <- newGenericMessage("heartbeat", data, r.nodeID)
			case <-r.shutdownChan:
				return
			}
		}
	}()

	Events.Subscribe(EVENT_QUIT, r.onQuiting)
}
