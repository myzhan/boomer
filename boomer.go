package boomer

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/asaskevich/EventBus"
)

// Events is the global event bus instance.
var Events = EventBus.New()

var defaultBoomer = &Boomer{}

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
type Boomer struct {
	masterHost string
	masterPort int

	hatchType   string
	mode        Mode
	rateLimiter RateLimiter
	slaveRunner *slaveRunner

	localRunner *localRunner
	hatchCount  int
	hatchRate   int

	cpuProfile         string
	cpuProfileDuration time.Duration

	memoryProfile         string
	memoryProfileDuration time.Duration

	outputs []Output
}

// NewBoomer returns a new Boomer.
func NewBoomer(masterHost string, masterPort int) *Boomer {
	return &Boomer{
		masterHost: masterHost,
		masterPort: masterPort,
		hatchType:  "asap",
		mode:       DistributedMode,
	}
}

// NewStandaloneBoomer returns a new Boomer, which can run without master.
func NewStandaloneBoomer(hatchCount int, hatchRate int) *Boomer {
	return &Boomer{
		hatchType:  "asap",
		hatchCount: hatchCount,
		hatchRate:  hatchRate,
		mode:       StandaloneMode,
	}
}

// SetRateLimiter allows user to use their own rate limiter.
// It must be called before the test is started.
func (b *Boomer) SetRateLimiter(rateLimiter RateLimiter) {
	b.rateLimiter = rateLimiter
}

// SetHatchType only accepts "asap" or "smooth".
// "asap" means spawning goroutines as soon as possible when the test is started.
// "smooth" means a constant pace.
func (b *Boomer) SetHatchType(hatchType string) {
	if hatchType != "asap" && hatchType != "smooth" {
		log.Printf("Wrong hatch-type, expected asap or smooth, was %s\n", hatchType)
		return
	}
	b.hatchType = hatchType
}

// SetMode only accepts boomer.DistributedMode and boomer.StandaloneMode.
func (b *Boomer) SetMode(mode Mode) {
	switch mode {
	case DistributedMode:
		b.mode = DistributedMode
	case StandaloneMode:
		b.mode = StandaloneMode
	default:
		log.Println("Invalid mode, ignored!")
	}
}

// AddOutput accepts outputs which implements the boomer.Output interface.
func (b *Boomer) AddOutput(o Output) {
	b.outputs = append(b.outputs, o)
}

// EnableCPUProfile will start cpu profiling after run.
func (b *Boomer) EnableCPUProfile(cpuProfile string, duration time.Duration) {
	b.cpuProfile = cpuProfile
	b.cpuProfileDuration = duration
}

// EnableMemoryProfile will start memory profiling after run.
func (b *Boomer) EnableMemoryProfile(memoryProfile string, duration time.Duration) {
	b.memoryProfile = memoryProfile
	b.memoryProfileDuration = duration
}

// Run accepts a slice of Task and connects to the locust master.
func (b *Boomer) Run(tasks ...*Task) {
	if b.cpuProfile != "" {
		err := StartCPUProfile(b.cpuProfile, b.cpuProfileDuration)
		if err != nil {
			log.Printf("Error starting cpu profiling, %v", err)
		}
	}
	if b.memoryProfile != "" {
		err := StartMemoryProfile(b.memoryProfile, b.memoryProfileDuration)
		if err != nil {
			log.Printf("Error starting memory profiling, %v", err)
		}
	}

	switch b.mode {
	case DistributedMode:
		b.slaveRunner = newSlaveRunner(b.masterHost, b.masterPort, tasks, b.rateLimiter, b.hatchType)
		for _, o := range b.outputs {
			b.slaveRunner.addOutput(o)
		}
		b.slaveRunner.run()
	case StandaloneMode:
		b.localRunner = newLocalRunner(tasks, b.rateLimiter, b.hatchCount, b.hatchType, b.hatchRate)
		for _, o := range b.outputs {
			b.localRunner.addOutput(o)
		}
		b.localRunner.run()
	default:
		log.Println("Invalid mode, expected boomer.DistributedMode or boomer.StandaloneMode")
	}
}

// RecordSuccess reports a success.
func (b *Boomer) RecordSuccess(requestType, name string, responseTime int64, responseLength int64) {
	if b.localRunner == nil && b.slaveRunner == nil {
		return
	}
	switch b.mode {
	case DistributedMode:
		b.slaveRunner.stats.requestSuccessChan <- &requestSuccess{
			requestType:    requestType,
			name:           name,
			responseTime:   responseTime,
			responseLength: responseLength,
		}
	case StandaloneMode:
		b.localRunner.stats.requestSuccessChan <- &requestSuccess{
			requestType:    requestType,
			name:           name,
			responseTime:   responseTime,
			responseLength: responseLength,
		}
	}
}

// RecordFailure reports a failure.
func (b *Boomer) RecordFailure(requestType, name string, responseTime int64, exception string) {
	if b.localRunner == nil && b.slaveRunner == nil {
		return
	}
	switch b.mode {
	case DistributedMode:
		b.slaveRunner.stats.requestFailureChan <- &requestFailure{
			requestType:  requestType,
			name:         name,
			responseTime: responseTime,
			error:        exception,
		}
	case StandaloneMode:
		b.localRunner.stats.requestFailureChan <- &requestFailure{
			requestType:  requestType,
			name:         name,
			responseTime: responseTime,
			error:        exception,
		}
	}
}

// Quit will send a quit message to the master.
func (b *Boomer) Quit() {
	Events.Publish("boomer:quit")
	var ticker = time.NewTicker(3 * time.Second)

	switch b.mode {
	case DistributedMode:
		// wait for quit message is sent to master
		select {
		case <-b.slaveRunner.client.disconnectedChannel():
			break
		case <-ticker.C:
			log.Println("Timeout waiting for sending quit message to master, boomer will quit any way.")
			break
		}
		b.slaveRunner.close()
	case StandaloneMode:
		b.localRunner.close()
	}
}

// Run tasks without connecting to the master.
func runTasksForTest(tasks ...*Task) {
	taskNames := strings.Split(runTasks, ",")
	for _, task := range tasks {
		if task.Name == "" {
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

	initLegacyEventHandlers()

	rateLimiter, err := createRateLimiter(maxRPS, requestIncreaseRate)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	defaultBoomer.SetRateLimiter(rateLimiter)
	defaultBoomer.masterHost = masterHost
	defaultBoomer.masterPort = masterPort
	defaultBoomer.hatchType = hatchType
	defaultBoomer.EnableMemoryProfile(memoryProfile, memoryProfileDuration)
	defaultBoomer.EnableCPUProfile(cpuProfile, cpuProfileDuration)

	defaultBoomer.Run(tasks...)

	quitByMe := false
	quitChan := make(chan bool)

	Events.SubscribeOnce("boomer:quit", func() {
		if !quitByMe {
			close(quitChan)
		}
	})

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-c:
		quitByMe = true
		defaultBoomer.Quit()
	case <-quitChan:
	}

	log.Println("shut down")
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
