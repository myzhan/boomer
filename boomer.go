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

	flag.Parse()

	czmq_client, err := NewZmqClient(*masterHost, *masterPort)
	var runner *Runner
	if err != nil {
		log.Println("ZMQ sockets failed to connect, trying pure Go sockets")
		client := NewSocketClient(*masterHost, *masterPort)
		runner = &Runner{
		Tasks: tasks,
		Client: client,
		NodeId: GetNodeId(),
		}

	}else{
		log.Println("ZMQ sockets connected")
		runner = &Runner{
		Tasks: tasks,
		Client: czmq_client,
		NodeId: GetNodeId(),
		}
	}



	Events.Subscribe("boomer:report_to_master", runner.onReportToMaster)
	Events.Subscribe("boomer:quit", runner.onQuiting)

	runner.GetReady()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	log.Println("Boomerx is listening to master(", fmt.Sprintf("%s:%d", *masterHost, *masterPort), ") press Ctrl+c to quit.")

	<- c
	Events.Publish("boomer:quit")

	// wait for quit message is sent to master
	<- DisconnectedFromServer
	log.Println("shut down")

}
