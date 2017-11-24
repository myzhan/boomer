package boomer

import (
	"flag"
)

type client interface {
	recv()
	send()
}

var fromServer = make(chan *message, 100)
var toServer = make(chan *message, 100)
var disconnectedFromServer = make(chan bool)

var masterHost *string
var masterPort *int
var rpc *string

func init() {
	masterHost = flag.String("master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing. Defaults to 127.0.0.1.")
	masterPort = flag.Int("master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing. Defaults to 5557.")
	rpc = flag.String("rpc", "zeromq", "Choose zeromq or tcp socket to communicate with master, don't mix them up.")
}
