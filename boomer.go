package boomer

import (
	"flag"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var masterHost string
var masterPort int

var maxRPS int64
var requestIncreaseRate string
var hatchType string
var runTasks string
var memoryProfile string
var memoryProfileDuration time.Duration
var cpuProfile string
var cpuProfileDuration time.Duration

var initted uint32
var initMutex = sync.Mutex{}

// Init boomer
func initBoomer() {
	if atomic.LoadUint32(&initted) == 1 {
		panic("Don't call boomer.Run() more than once.")
	}

	initEvents()
	defaultStats.start()

	// done
	atomic.StoreUint32(&initted, 1)
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

// Run accepts a slice of Task and connects
// to a locust master.
func Run(tasks ...*Task) {
	if !flag.Parsed() {
		flag.Parse()
	}

	if runTasks != "" {
		runTasksForTest(tasks...)
		return
	}

	// support go version below 1.5
	runtime.GOMAXPROCS(runtime.NumCPU())

	// init boomer
	initMutex.Lock()
	initBoomer()
	initMutex.Unlock()

	var rateLimiter rateLimiter
	if requestIncreaseRate != "-1" {
		if maxRPS > 0 {
			log.Println("The max RPS that boomer may generate is limited to", maxRPS, "with a increase rate", requestIncreaseRate)
			rateLimiter = newWarmUpRateLimiter(maxRPS, requestIncreaseRate, time.Second)
		} else {
			log.Println("The max RPS that boomer may generate is limited by a increase rate", requestIncreaseRate)
			rateLimiter = newWarmUpRateLimiter(math.MaxInt64, requestIncreaseRate, time.Second)
		}
	} else {
		if maxRPS > 0 {
			log.Println("The max RPS that boomer may generate is limited to", maxRPS)
			rateLimiter = newStableRateLimiter(maxRPS, time.Second)
		}
	}

	runner := newRunner(tasks, rateLimiter, hatchType)
	runner.masterHost = masterHost
	runner.masterPort = masterPort
	runner.getReady()

	if memoryProfile != "" {
		startMemoryProfile(memoryProfile, memoryProfileDuration)
	}

	if cpuProfile != "" {
		startCPUProfile(cpuProfile, cpuProfileDuration)
	}

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	<-c
	Events.Publish("boomer:quit")

	// wait for quit message is sent to master
	<-runner.client.disconnectedChannel()
	log.Println("shut down")
}

func startMemoryProfile(file string, duration time.Duration) {
	f, err := os.Create(file)
	if err != nil {
		log.Fatal(err)
	}
	time.AfterFunc(duration, func() {
		err = pprof.WriteHeapProfile(f)
		if err != nil {
			log.Println(err)
			return
		}
		f.Close()
		log.Println("Stop memory profiling after", duration)
	})
}

func startCPUProfile(file string, duration time.Duration) {
	f, err := os.Create(file)
	if err != nil {
		log.Fatal(err)
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Println(err)
		f.Close()
		return
	}

	time.AfterFunc(duration, func() {
		pprof.StopCPUProfile()
		f.Close()
		log.Println("Stop CPU profiling after", duration)
	})
}

func init() {
	flag.Int64Var(&maxRPS, "max-rps", 0, "Max RPS that boomer can generate, disabled by default.")
	flag.StringVar(&requestIncreaseRate, "request-increase-rate", "-1", "Request increase rate, disabled by default.")
	flag.StringVar(&hatchType, "hatch-type", "asap", "How to create goroutines according to hatch rate, 'asap' will do it as soon as possible while 'smooth' means a constant pace.")
	flag.StringVar(&runTasks, "run-tasks", "", "Run tasks without connecting to the master, multiply tasks is separated by comma. Usually, it's for debug purpose.")
	flag.StringVar(&masterHost, "master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing.")
	flag.IntVar(&masterPort, "master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing.")
	flag.StringVar(&memoryProfile, "mem-profile", "", "Enable memory profiling.")
	flag.DurationVar(&memoryProfileDuration, "mem-profile-duration", 30*time.Second, "Memory profile duration.")
	flag.StringVar(&cpuProfile, "cpu-profile", "", "Enable CPU profiling.")
	flag.DurationVar(&cpuProfileDuration, "cpu-profile-duration", 30*time.Second, "CPU profile duration.")
}
