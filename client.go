package boomer

type client interface {
	recv()
	send()
}

var fromMaster = make(chan *message, 100)
var toMaster = make(chan *message, 100)
var disconnectedFromMaster = make(chan bool)

var masterHost string
var masterPort int
var rpc string