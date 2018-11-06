# boomer [![Build Status](https://travis-ci.org/myzhan/boomer.svg?branch=master)](https://travis-ci.org/myzhan/boomer) [![Go Report Card](https://goreportcard.com/badge/github.com/myzhan/boomer)](https://goreportcard.com/report/github.com/myzhan/boomer) [![Coverage Status](https://codecov.io/gh/myzhan/boomer/branch/master/graph/badge.svg)](https://codecov.io/gh/myzhan/boomer)

## Links

* Locust Website: <a href="http://locust.io">locust.io</a>
* Locust Documentation: <a href="http://docs.locust.io">docs.locust.io</a>

## Description

Boomer is a better load generator for locust, written in golang. It can spawn thousands of goroutines to run your code concurrently.

It will listen and report to the locust master automatically, your test results will be displayed on the master's web UI.


## Install

```bash
go get github.com/myzhan/boomer
```

### Zeromq support

Boomer use gomq by default, which is a pure Go implementation of the ZeroMQ.

Becase of the instability of gomq, you can switch to [goczmq](https://github.com/zeromq/goczmq).

```bash
# use gomq
go build -o a.out main.go
# use goczmq
go build -tags 'goczmq' -o a.out main.go
```

If you fail to compile boomer with gomq, try to update gomq fisrt.

```bash
go get -u github.com/zeromq/gomq
```

## Examples(main.go)
This is a example of boomer's API. You can find more in "examples" directory.

```go
package main


import "github.com/myzhan/boomer"
import "time"


func foo(){

    start := boomer.Now()
    time.Sleep(100 * time.Millisecond)
    elapsed := boomer.Now() - start

    /*
    Report your test result as a success, if you write it in locust, it will looks like this
    events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
    */
    boomer.Events.Publish("request_success", "http", "foo", elapsed, int64(10))
}


func bar(){

    start := boomer.Now()
    time.Sleep(100 * time.Millisecond)
    elapsed := boomer.Now() - start

    /*
    Report your test result as a failure, if you write it in locust, it will looks like this
    events.request_failure.fire(request_type="udp", name="bar", response_time=100, exception=Exception("udp error"))
    */
    boomer.Events.Publish("request_failure", "udp", "bar", elapsed, "udp error")
}


func main(){

    task1 := &boomer.Task{
        Name: "foo",
        Weight: 10,
        Fn: foo,
    }

    task2 := &boomer.Task{
        Name: "bar",
        Weight: 20,
        Fn: bar,
    }

    boomer.Run(task1, task2)

}
```

## Usage

For debug purpose, you can run tasks without connecting to the master.

```bash
go build -o a.out main.go
./a.out --run-tasks foo,bar
```

If you want to limit max RPS that a single instance of boomer can generate.

```bash
go build -o a.out main.go
./a.out --max-rps 10000
```

If you want the RPS increase from zero to max-rps or infinity.

```
go build -o a.out main.go
# The default interval is 1 second
./a.out --request-increase-rate 10
# Change the interval to 1 minute
# Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"
./a.out --request-increase-rate 10/1m
```

If master is listening on zeromq socket.

```bash
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
go build -o a.out main.go
./a.out --master-host=127.0.0.1 --master-port=5557 --rpc=zeromq
```

If master is listening on tcp socket.

```bash
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
go build -o a.out main.go
./a.out --master-host=127.0.0.1 --master-port=5557 --rpc=socket
```

So far, dummy.py is necessary when starting a master, because locust needs such a file.

Don't worry, dummy.py has nothing to do with your test.

## Contributing

If you are enjoying boomer and willing to add new features to it, you are welcome.

Also, good examples are welcome!!!

## License

Open source licensed under the MIT license (see _LICENSE_ file for details).
