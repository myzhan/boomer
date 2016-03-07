# Boomer

## Links

* Website: <a href="http://locust.io">locust.io</a>
* Documentation: <a href="http://docs.locust.io">docs.locust.io</a>

## Description

Boomer is a locust slave runner written in golang. Just write your test scenarios in golang functions, and tell Boomer to run them.
Boomer will report to the locust master automatically, and your test result will be displayed on the master's web UI.


## Install
First of all, install [goczmq](https://github.com/zeromq/goczmq#building-from-source-linux) by following its installation guide. Then you get boomer.

```bash
go get github.com/myzhan/boomer
```


## Sample(main.go)
```go
package main


import "github.com/myzhan/boomer"
import "time"


func foo(){
	time.Sleep(100 * time.Millisecond)
	/*
    Report your test result as a success, if you write it in python, it will looks like this
    events.request_success.fire(request_type="http", name="foo", response_time=100.0, response_length=10)
    */
    boomer.Events.Publish("request_success", "foo", "http", 100.0, int64(10))
}


func bar(){
	time.Sleep(100 * time.Millisecond)
	/*
    Report your test result as a failure, if you write it in python, it will looks like this
    events.request_failure.fire(request_type="udp", name="bar", response_time=100.0, exception=Exception("udp error"))
    */
    boomer.Events.Publish("request_failure", "bar", "udp", 100.0, "udp error")
}


func main(){

	task1 := &boomer.Task{
		Weight: 10,
		Fn: foo,
	}

	task2 := &boomer.Task{
		Weight: 20,
		Fn: bar,
	}


	boomer.Run(task1, task2)

}
```

## Usage
If master is listening on zeromq socket.
```bash
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
go run main.go --master-host=127.0.0.1 --master-port=5557 --rpc=zeromq
```
If master is listening on tcp socket.
```bash
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
go run main.go --master-host=127.0.0.1 --master-port=5557 --rpc=socket
```


## License

Open source licensed under the MIT license (see _LICENSE_ file for details).
