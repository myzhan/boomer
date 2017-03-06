// +build !zeromq

package boomer

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func Run(tasks ...*Task) {

	// support go version below 1.5
	runtime.GOMAXPROCS(runtime.NumCPU())

	masterHost := flag.String("master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	masterPort := flag.Int("master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")

	flag.Parse()

	log.Println("Boomer is built without zeromq support. We recommend you to install the goczmq package, and build Boomer with zeromq when running in distributed mode.")

	var message string
	var runner *Runner

	client := NewSocketClient(*masterHost, *masterPort)
	runner = &Runner{
		Tasks:  tasks,
		Client: client,
		NodeId: GetNodeId(),
	}
	message = fmt.Sprintf("Boomer is connected to master(%s:%d) press Ctrl+c to quit.", *masterHost, *masterPort)

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
