package boomer

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"log"
	"runtime"
)


func Run(tasks ... *Task) {


	// support go version below 1.5
	runtime.GOMAXPROCS(runtime.NumCPU())

	masterHost := flag.String("master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	masterPort := flag.Int("master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")

	flag.Parse()

	client := NewSocketClient(*masterHost, *masterPort)

	runner := &Runner{
		Tasks: tasks,
		Client: client,
		NodeId: GetNodeId(),
	}

	Events.Subscribe("boomer:report_to_master", runner.onReportToMaster)
	Events.Subscribe("boomer:quit", runner.onQuiting)

	runner.GetReady()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	<- c
	Events.Publish("boomer:quit")

	// wait for quit message is sent to master
	<- DisconnectedFromServer
	log.Println("shut down")

}
