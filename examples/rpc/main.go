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

// 修改为要压测的服务接口
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

	// 构建请求对象
	request := &HelloRequest{}
	request.Name = req.Name

	// 初始化响应对象
	resp := new(HelloReply)
	err := client.Call(request, resp)

	elapsed := time.Since(startTime)

	if err != nil {
		if verbose {
			log.Printf("%v\n", err)
		}
		boomer.RecordFailure("rpc", "error", 0.0, err.Error())
	} else {
		// 添加自定义断言
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
	// json反序列化
	err := json.Unmarshal([]byte(reqJSONStr), &req)

	if nil != err {
		log.Printf("json unmarshal error")
		return
	}

	client = grequester.NewRequester(addr, service, method, timeout, poolsize)

	task := &boomer.Task{
		Name:   "rpcReq",
		Weight: 10,
		Fn:     rpcReq,
	}

	boomer.Run(task)
}
