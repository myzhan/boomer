package boomer

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var defaultBoomer = &Boomer{logger: log.Default()}

// Mode is the running mode of boomer, both standalone and distributed are supported.
type Mode int

const (
	// DistributedMode requires connecting to a master.
	DistributedMode Mode = iota
	// StandaloneMode will run without a master.
	StandaloneMode
)

// A Boomer is used to run tasks.
// This type is exposed, so users can create and control a Boomer instance programmatically.
// A non-nil logger is supposed to be set.
type Boomer struct {
	masterHost  string
	masterPort  int
	mode        Mode
	rateLimiter RateLimiter
	mu          sync.RWMutex
	slaveRunner *slaveRunner

	localRunner *localRunner
	spawnCount  int
	spawnRate   float64

	cpuProfileFile     string
	cpuProfileDuration time.Duration

	memoryProfileFile     string
	memoryProfileDuration time.Duration

	outputs []Output

	logger *log.Logger

	// quitting is guarded by mu, it makes Quit idempotent.
	quitting bool

	// recordStats caches the stats of the runner which is running, so that
	// recording a request doesn't take the lock on every call.
	recordStats atomic.Pointer[requestStats]
}

// NewBoomer returns a new Boomer.
func NewBoomer(masterHost string, masterPort int) *Boomer {
	return &Boomer{
		masterHost: masterHost,
		masterPort: masterPort,
		mode:       DistributedMode,
		logger:     log.Default(),
	}
}

// NewStandaloneBoomer returns a new Boomer, which can run without master.
func NewStandaloneBoomer(spawnCount int, spawnRate float64) *Boomer {
	return &Boomer{
		spawnCount: spawnCount,
		spawnRate:  spawnRate,
		mode:       StandaloneMode,
		logger:     log.Default(),
	}
}

// WithLogger allows user to use their own logger.
// If the logger is nil, it will not take effect.
func (b *Boomer) WithLogger(logger *log.Logger) *Boomer {
	if logger == nil {
		return b
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logger = logger
	if b.slaveRunner != nil {
		b.slaveRunner.setLogger(logger)
	}
	if b.localRunner != nil {
		b.localRunner.setLogger(logger)
	}
	return b
}

// SetRateLimiter allows user to use their own rate limiter.
// It must be called before the test is started.
func (b *Boomer) SetRateLimiter(rateLimiter RateLimiter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rateLimiter = rateLimiter
}

// SetMode only accepts boomer.DistributedMode and boomer.StandaloneMode.
func (b *Boomer) SetMode(mode Mode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch mode {
	case DistributedMode:
		b.mode = DistributedMode
	case StandaloneMode:
		b.mode = StandaloneMode
	default:
		b.logger.Println("Invalid mode, ignored!")
	}
}

// AddOutput accepts outputs which implements the boomer.Output interface.
func (b *Boomer) AddOutput(o Output) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.outputs = append(b.outputs, o)
}

// EnableCPUProfile will start cpu profiling after run.
func (b *Boomer) EnableCPUProfile(cpuProfileFile string, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cpuProfileFile = cpuProfileFile
	b.cpuProfileDuration = duration
}

// EnableMemoryProfile will start memory profiling after run.
func (b *Boomer) EnableMemoryProfile(memoryProfileFile string, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.memoryProfileFile = memoryProfileFile
	b.memoryProfileDuration = duration
}

// Run accepts a slice of Task and connects to the locust master.
func (b *Boomer) Run(tasks ...*Task) {
	b.mu.Lock()
	b.quitting = false
	mode := b.mode
	rateLimiter := b.rateLimiter
	logger := b.logger
	cpuProfileFile, cpuProfileDuration := b.cpuProfileFile, b.cpuProfileDuration
	memoryProfileFile, memoryProfileDuration := b.memoryProfileFile, b.memoryProfileDuration
	spawnCount, spawnRate := b.spawnCount, b.spawnRate
	masterHost, masterPort := b.masterHost, b.masterPort
	outputs := make([]Output, len(b.outputs))
	copy(outputs, b.outputs)
	b.mu.Unlock()

	if logger == nil {
		logger = log.Default()
	}

	// A nil task would be executed by the workers, guard against it.
	runnableTasks := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		runnableTasks = append(runnableTasks, task)
	}
	if len(runnableTasks) == 0 {
		logger.Println("No tasks to run, boomer will not start.")
		return
	}

	if cpuProfileFile != "" {
		err := StartCPUProfile(cpuProfileFile, cpuProfileDuration)
		if err != nil {
			logger.Printf("Error starting cpu profiling, %v", err)
		}
	}
	if memoryProfileFile != "" {
		err := StartMemoryProfile(memoryProfileFile, memoryProfileDuration)
		if err != nil {
			logger.Printf("Error starting memory profiling, %v", err)
		}
	}

	switch mode {
	case DistributedMode:
		sr := newSlaveRunner(masterHost, masterPort, runnableTasks, rateLimiter)
		sr.setLogger(logger)
		logger.Println("new slave runner")
		for _, o := range outputs {
			sr.addOutput(o)
		}
		b.mu.Lock()
		b.slaveRunner = sr
		b.recordStats.Store(sr.stats)
		b.mu.Unlock()
		sr.run()
	case StandaloneMode:
		lr := newLocalRunner(runnableTasks, rateLimiter, spawnCount, spawnRate)
		lr.setLogger(logger)
		logger.Println("new local runner")
		for _, o := range outputs {
			lr.addOutput(o)
		}
		b.mu.Lock()
		b.localRunner = lr
		b.recordStats.Store(lr.stats)
		b.mu.Unlock()
		lr.run()
	default:
		logger.Println("Invalid mode, expected boomer.DistributedMode or boomer.StandaloneMode")
	}
}

// runnerStats returns the stats instance of the runner matching the current mode,
// or nil if boomer has not been started.
func (b *Boomer) runnerStats() (stats *requestStats) {
	if stats = b.recordStats.Load(); stats != nil {
		return stats
	}
	// No runner has been started yet, or a runner was injected directly.
	b.mu.RLock()
	defer b.mu.RUnlock()

	switch b.mode {
	case DistributedMode:
		if sr := b.slaveRunner; sr != nil {
			stats = sr.stats
		}
	case StandaloneMode:
		if lr := b.localRunner; lr != nil {
			stats = lr.stats
		}
	}
	return stats
}

// RecordSuccess reports a success.
func (b *Boomer) RecordSuccess(requestType, name string, responseTime int64, responseLength int64) {
	stats := b.runnerStats()
	if stats == nil {
		return
	}
	msg := &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   responseTime,
		responseLength: responseLength,
	}
	// The channel is buffered and its consumer is a single goroutine, so this
	// pre-check keeps the common case as cheap as a plain send. A select is only
	// used when the buffer is full, where it can also give up on a closed stats.
	ch := stats.requestSuccessChan
	if len(ch) < cap(ch) {
		ch <- msg
		return
	}
	select {
	case ch <- msg:
	case <-stats.shutdownChan:
		// stats has been closed, drop the record instead of blocking forever.
	}
}

// RecordFailure reports a failure.
func (b *Boomer) RecordFailure(requestType, name string, responseTime int64, exception string) {
	stats := b.runnerStats()
	if stats == nil {
		return
	}
	msg := &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: responseTime,
		error:        exception,
	}
	// See RecordSuccess for why the send is guarded like this.
	ch := stats.requestFailureChan
	if len(ch) < cap(ch) {
		ch <- msg
		return
	}
	select {
	case ch <- msg:
	case <-stats.shutdownChan:
		// stats has been closed, drop the record instead of blocking forever.
	}
}

// SendCustomMessage sends a custom message to the master.
func (b *Boomer) SendCustomMessage(messageType string, data interface{}) {
	b.mu.RLock()
	mode := b.mode
	sr := b.slaveRunner
	lr := b.localRunner
	b.mu.RUnlock()

	switch mode {
	case DistributedMode:
		if sr != nil {
			sr.sendCustomMessage(messageType, data)
		}
	case StandaloneMode:
		if lr != nil {
			lr.sendCustomMessage(messageType, data)
		}
	}
}

// Quit will send a quit message to the master.
func (b *Boomer) Quit() {
	b.mu.Lock()
	if b.quitting {
		b.mu.Unlock()
		return
	}
	b.quitting = true
	mode := b.mode
	sr := b.slaveRunner
	lr := b.localRunner
	b.mu.Unlock()

	Events.Publish(EVENT_QUIT)

	switch mode {
	case DistributedMode:
		if sr == nil {
			return
		}
		if sr.client == nil {
			sr.shutdown()
			return
		}
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		// wait for quit message is sent to master
		select {
		case <-sr.client.disconnectedChannel():
		case <-ticker.C:
			if b.logger != nil {
				b.logger.Println("Timeout waiting for sending quit message to master, boomer will quit any way.")
			}
		}
		sr.shutdown()
	case StandaloneMode:
		if lr != nil {
			lr.shutdown()
		}
	}
}

// Run tasks without connecting to the master.
func runTasksForTest(tasks ...*Task) {
	taskNames := strings.Split(runTasks, ",")
	for _, task := range tasks {
		if task == nil || task.Name == "" {
			continue
		} else {
			for _, name := range taskNames {
				if name == task.Name {
					log.Println("Running " + task.Name)
					task.Fn()
				}
			}
		}
	}
}

// Run accepts a slice of Task and connects to a locust master.
// It's a convenience function to use the defaultBoomer.
func Run(tasks ...*Task) {
	if !flag.Parsed() {
		flag.Parse()
	}

	if runTasks != "" {
		runTasksForTest(tasks...)
		return
	}

	if len(tasks) == 0 {
		log.Fatal("No tasks to run, boomer will exit.")
	}

	initLegacyEventHandlers()

	rateLimiter, err := createRateLimiter(maxRPS, requestIncreaseRate)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	defaultBoomer.SetRateLimiter(rateLimiter)
	defaultBoomer.masterHost = masterHost
	defaultBoomer.masterPort = masterPort
	defaultBoomer.EnableMemoryProfile(memoryProfileFile, memoryProfileDuration)
	defaultBoomer.EnableCPUProfile(cpuProfileFile, cpuProfileDuration)

	// Subscribe before running, otherwise a quit message arriving right after connecting
	// would be missed and Run would block forever waiting for it.
	var quitByMe atomic.Bool
	quitChan := make(chan bool, 1)
	Events.SubscribeOnce(EVENT_QUIT, func() {
		if !quitByMe.Load() {
			select {
			case quitChan <- true:
			default:
			}
		}
	})

	defaultBoomer.Run(tasks...)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-c:
		quitByMe.Store(true)
		defaultBoomer.Quit()
	case <-quitChan:
	}

	log.Println("shutdown")
}

// RecordSuccess reports a success.
// It's a convenience function to use the defaultBoomer.
func RecordSuccess(requestType, name string, responseTime int64, responseLength int64) {
	defaultBoomer.RecordSuccess(requestType, name, responseTime, responseLength)
}

// RecordFailure reports a failure.
// It's a convenience function to use the defaultBoomer.
func RecordFailure(requestType, name string, responseTime int64, exception string) {
	defaultBoomer.RecordFailure(requestType, name, responseTime, exception)
}
