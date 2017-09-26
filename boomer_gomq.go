// +build gomq

package boomer

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

func Run(tasks ...*Task) {

	// support go version below 1.5
	runtime.GOMAXPROCS(runtime.NumCPU())

	if !flag.Parsed() {
		flag.Parse()
	}

	if *runTasks != "" {
		// Run tasks without connecting to the master.
		taskNames := strings.Split(*runTasks, ",")
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

	log.Println("Boomer is built with zeromq support.")

	var message string
	var r *runner
	if *rpc == "zeromq" {
		client := newZmqClient(*masterHost, *masterPort)
		r = &runner{
			tasks:  tasks,
			client: client,
			nodeId: getNodeId(),
		}
		message = fmt.Sprintf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.", *masterHost, *masterPort, *masterPort+1)
	} else if *rpc == "socket" {
		client := newSocketClient(*masterHost, *masterPort)
		r = &runner{
			tasks:  tasks,
			client: client,
			nodeId: getNodeId(),
		}
		message = fmt.Sprintf("Boomer is connected to master(%s:%d) press Ctrl+c to quit.", *masterHost, *masterPort)
	} else {
		log.Fatal("Unknown rpc type:", *rpc)
	}

	Events.Subscribe("boomer:quit", r.onQuiting)

	r.getReady()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	log.Println(message)

	<-c
	Events.Publish("boomer:quit")

	// wait for quit message is sent to master
	<-disconnectedFromServer
	log.Println("shut down")

}

var masterHost *string
var masterPort *int
var rpc *string
var runTasks *string

func init() {
	masterHost = flag.String("master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	masterPort = flag.Int("master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")
	rpc = flag.String("rpc", "zeromq", "Choose zeromq or tcp socket to communicate with master, don't mix them up.")
	runTasks = flag.String("run-tasks", "", "Run tasks without connecting to the master, multiply tasks is seperated by comma. Usually, it's for debug purpose.")
}
