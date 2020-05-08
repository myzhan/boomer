package main

import (
	"encoding/json"
	"flag"
	"log"
	"time"

	"github.com/bugVanisher/grequester"
	"github.com/myzhan/boomer"
)

var verbose = false

// change to your own service and method
var service = "helloworld.Greeter"
var method = "SayHello"
var timeout uint = 3000
var poolsize = 200

var (
	addr       string
	reqJSONStr string
	client     *grequester.Requester
	req        *HelloRequest
)

func rpcReq() {
	startTime := time.Now()

	// make the request
	request := &HelloRequest{}
	request.Name = req.Name

	// init the response
	resp := new(HelloReply)
	err := client.Call(request, resp)

	elapsed := time.Since(startTime)

	if err != nil {
		if verbose {
			log.Printf("%v\n", err)
		}
		boomer.RecordFailure("rpc", "error", 0.0, err.Error())
	} else {
		// make your assertion
		boomer.RecordSuccess("rpc", "succ",
			elapsed.Nanoseconds()/int64(time.Millisecond), int64(len(resp.String())))
		if verbose {
			if err != nil {
				log.Printf("%v\n", err)
			} else {
				log.Printf("Resp Length: %d\n", len(resp.String()))
				log.Println(resp.String())
			}
		}
	}
}

func main() {
	flag.StringVar(&addr, "a", "", "ip:port")
	flag.StringVar(&reqJSONStr, "r", "{}", "request message in json form")
	flag.Parse()

	log.Printf(reqJSONStr)
	// json unserialize, input different parameters
	err := json.Unmarshal([]byte(reqJSONStr), &req)

	if nil != err {
		log.Printf("json unmarshal error")
		return
	}

	// init requester
	client = grequester.NewRequester(addr, service, method, timeout, poolsize)

	task := &boomer.Task{
		Name:   "rpcReq",
		Weight: 10,
		Fn:     rpcReq,
	}

	boomer.Run(task)
}
