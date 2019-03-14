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

var defaultBoomer *Boomer

// A Boomer is used to run tasks.
// This type is exposed, so users can create and control a Boomer instance programmatically.
type Boomer struct {
	masterHost string
	masterPort int

	hatchType   string
	rateLimiter RateLimiter
	runner      *runner
}

// NewBoomer returns a new Boomer.
func NewBoomer(masterHost string, masterPort int) *Boomer {
	return &Boomer{
		masterHost: masterHost,
		masterPort: masterPort,
		hatchType:  "asap",
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

func (b *Boomer) setRunner(runner *runner) {
	b.runner = runner
}

// Run accepts a slice of Task and connects to the locust master.
func (b *Boomer) Run(tasks ...*Task) {
	b.runner = newRunner(tasks, b.rateLimiter, b.hatchType)
	b.runner.masterHost = b.masterHost
	b.runner.masterPort = b.masterPort
	b.runner.getReady()
}

// RecordSuccess reports a success.
func (b *Boomer) RecordSuccess(requestType, name string, responseTime int64, responseLength int64) {
	b.runner.stats.requestSuccessChannel <- &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   responseTime,
		responseLength: responseLength,
	}
}

// RecordFailure reports a failure.
func (b *Boomer) RecordFailure(requestType, name string, responseTime int64, exception string) {
	b.runner.stats.requestFailureChannel <- &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: responseTime,
		error:        exception,
	}
}

// Quit will send a quit message to the master.
func (b *Boomer) Quit() {
	Events.Publish("boomer:quit")
	var ticker = time.NewTicker(3 * time.Second)

	// wait for quit message is sent to master
	select {
	case <-b.runner.client.disconnectedChannel():
		break
	case <-ticker.C:
		log.Println("Timeout waiting for sending quit message to master, boomer will quit any way.")
		break
	}

	b.runner.close()
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

	defaultBoomer = NewBoomer(masterHost, masterPort)
	initLegacyEventHandlers()

	if memoryProfile != "" {
		StartMemoryProfile(memoryProfile, memoryProfileDuration)
	}

	if cpuProfile != "" {
		StartCPUProfile(cpuProfile, cpuProfileDuration)
	}

	rateLimiter, err := createRateLimiter(maxRPS, requestIncreaseRate)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	defaultBoomer.SetRateLimiter(rateLimiter)
	defaultBoomer.hatchType = hatchType

	defaultBoomer.Run(tasks...)

	quitByMe := false
	Events.Subscribe("boomer:quit", func() {
		if !quitByMe {
			log.Println("shut down")
			os.Exit(0)
		}
	})

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	<-c
	quitByMe = true
	defaultBoomer.Quit()

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
