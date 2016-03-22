// +build zeromq

package boomer

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"log"
	"runtime"
	"fmt"
)


func Run(tasks ... *Task) {


	// support go version below 1.5
	runtime.GOMAXPROCS(runtime.NumCPU())

	masterHost := flag.String("master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	masterPort := flag.Int("master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")
	rpc := flag.String("rpc", "zeromq", "Choose zeromq or tcp socket to communicate with master, don't mix them up.")

	flag.Parse()

	log.Println("Boomer is built with zeromq support.")

	var message string
	var runner *Runner
	if *rpc == "zeromq" {
		client := NewZmqClient(*masterHost, *masterPort)
		runner = &Runner{
			Tasks: tasks,
			Client: client,
			NodeId: GetNodeId(),
		}
		message = fmt.Sprintf("Boomer is connected to master(%s:%d|%d) press Ctrl+c to quit.", *masterHost, *masterPort, *masterPort + 1)
	}else if *rpc == "socket" {
		client := NewSocketClient(*masterHost, *masterPort)
		runner = &Runner{
			Tasks: tasks,
			Client: client,
			NodeId: GetNodeId(),
		}
		message = fmt.Sprintf("Boomer is connected to master(%s:%d) press Ctrl+c to quit.", *masterHost, *masterPort)
	}else {
		log.Fatal("Unknown rpc type:", *rpc)
	}

	Events.Subscribe("boomer:report_to_master", runner.onReportToMaster)
	Events.Subscribe("boomer:quit", runner.onQuiting)

	runner.GetReady()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	log.Println(message)

	<-c
	Events.Publish("boomer:quit")

	// wait for quit message is sent to master
	<-DisconnectedFromServer
	log.Println("shut down")

}
