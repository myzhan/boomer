package boomer

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var runTasks string
var maxRPS int64
var requestIncreaseRate string
var requestIncreaseStep int64
var requestIncreaseInterval time.Duration
var currentRPSThreshold = int64(0)
var rpsThreshold int64
var rpsControlEnabled = false
var rpsControlChannel = make(chan bool)
var rpsControllerQuitChannel = make(chan bool)

var memoryProfile string
var cpuProfile string

var initted uint32
var initMutex = sync.Mutex{}

// Init boomer
func initBoomer() {

	// support go version below 1.5
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Int64Var(&maxRPS, "max-rps", 0, "Max RPS that boomer can generate, disabled by default.")
	flag.StringVar(&requestIncreaseRate, "request-increase-rate", "-1", "Request increase rate, disabled by default.")
	flag.StringVar(&runTasks, "run-tasks", "", "Run tasks without connecting to the master, multiply tasks is separated by comma. Usually, it's for debug purpose.")
	flag.StringVar(&masterHost, "master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	flag.IntVar(&masterPort, "master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")
	flag.StringVar(&rpc, "rpc", "zeromq", "Choose zeromq or tcp socket to communicate with master, don't mix them up.")
	flag.StringVar(&memoryProfile, "mem-profile", "", "Collect runtime heap profile after 30 seconds.")
	flag.StringVar(&cpuProfile, "cpu-profile", "", "Enable CPU profiling for 30 seconds.")

	if !flag.Parsed() {
		flag.Parse()
	}

	if maxRPS > 0 || requestIncreaseRate != "-1" {
		rpsControlEnabled = true
		if maxRPS > 0 {
			log.Println("Max RPS that boomer may generate is limited to", maxRPS)
		}
		if requestIncreaseRate != "-1" {
			log.Println("Request increase rate is", requestIncreaseRate)
			if strings.Contains(requestIncreaseRate, "/") {
				tmp := strings.Split(requestIncreaseRate, "/")
				if len(tmp) != 2 {
					log.Fatalf("Wrong format of requestIncreaseRate, %s", requestIncreaseRate)
				}
				step, err := strconv.ParseInt(tmp[0], 10, 64)
				if err != nil {
					log.Fatalf("Failed to parse requestIncreaseRate, %v", err)
				}
				requestIncreaseStep = step
				requestIncreaseInterval, err = time.ParseDuration(tmp[1])
				if err != nil {
					log.Fatalf("Failed to parse requestIncreaseRate, %v", err)
				}
			} else {
				step, err := strconv.ParseInt(requestIncreaseRate, 10, 64)
				if err != nil {
					log.Fatalf("Failed to parse requestIncreaseRate, %v", err)
				}
				requestIncreaseStep = step
				requestIncreaseInterval = time.Second
			}
		}
	}

	initEvents()
	initStats()

	// done
	atomic.StoreUint32(&initted, 1)
}

// Run accepts a slice of Task and connects
// to a locust master.
func Run(tasks ...*Task) {

	if atomic.LoadUint32(&initted) == 1 {
		panic("Don't call boomer.Run() more than once.")
	}

	// init boomer
	initMutex.Lock()
	initBoomer()
	initMutex.Unlock()

	if runTasks != "" {
		// Run tasks without connecting to the master.
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
		return
	}

	var r *runner
	client := newClient()
	r = &runner{
		tasks:  tasks,
		client: client,
		nodeID: getNodeID(),
	}

	_startRunner(r)

}

func RunClients(tasks TaskClient) {

	if atomic.LoadUint32(&initted) == 1 {
		panic("Don't call boomer.Run() more than once.")
	}

	// init boomer
	initMutex.Lock()
	initBoomer()
	initMutex.Unlock()

	if runTasks != "" {
		c := tasks.New()
		if !c.OnStart(1) {
			log.Println("OnStart fail")
			return
		}
		// Run tasks without connecting to the master.
		taskNames := strings.Split(runTasks, ",")
		for _, task := range c.Tasks() {
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
		return
	}

	var r *runner
	client := newClient()
	r = &runner{
		taskClients: tasks,
		client:      client,
		nodeID:      getNodeID(),
	}
	_startRunner(r)

}

func _startRunner(r *runner) {
	Events.Subscribe("boomer:quit", r.onQuiting)

	r.getReady()

	if memoryProfile != "" {
		startMemoryProfile()
	}

	if cpuProfile != "" {
		startCPUProfile()
	}

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	<-c
	Events.Publish("boomer:quit")

	// wait for quit message is sent to master
	<-disconnectedFromMaster
	log.Println("shut down")
}

func startMemoryProfile() {
	f, err := os.Create(memoryProfile)
	if err != nil {
		log.Fatal(err)
	}
	time.AfterFunc(30*time.Second, func() {
		err = pprof.WriteHeapProfile(f)
		if err != nil {
			log.Println(err)
			return
		}
		f.Close()
		log.Println("Stop memory profiling after 30 seconds")
	})
}

func startCPUProfile() {
	f, err := os.Create(cpuProfile)
	if err != nil {
		log.Fatal(err)
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Println(err)
		f.Close()
		return
	}

	time.AfterFunc(30*time.Second, func() {
		pprof.StopCPUProfile()
		f.Close()
		log.Println("Stop CPU profiling after 30 seconds")
	})
}
