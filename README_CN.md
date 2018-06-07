# boomer [![Build Status](https://travis-ci.org/myzhan/boomer.svg?branch=master)](https://travis-ci.org/myzhan/boomer) [![Go Report Card](https://goreportcard.com/badge/github.com/myzhan/boomer)](https://goreportcard.com/report/github.com/myzhan/boomer)

## boomer是什么？

boomer 完整地实现了 locust 的通讯协议，运行在 slave 模式下，用 goroutine 来执行用户提供的测试函数，然后将测试结果上报给运行在 master 模式下的 locust。

与 locust 原生的实现相比，解决了两个问题。一是单台施压机上，能充分利用多个 CPU 核心来施压，二是再也不用提防阻塞 IO 操作导致 gevent 阻塞。

## 安装

```bash
go get github.com/myzhan/boomer
```

### zeromq 支持
boomer 默认使用 gomq，一个纯 Go 语言实现的 ZeroMQ 客户端。

由于 gomq 还不稳定，可以改用 [goczmq](https://github.com/zeromq/goczmq)。

```bash
# 默认使用 gomq
go build -o a.out main.go
# 使用 goczmq
go build -tags 'goczmq' -o a.out main.go
```

如果使用 gomq 编译失败，先尝试更新 gomq 的版本。

```bash
go get -u github.com/zeromq/gomq
```

## 例子(main.go)
下面演示一下 boomer 的 API，可以在 examples 目录下找到更多的例子。

```go
package main


import "github.com/myzhan/boomer"
import "time"


func foo(){

    start := boomer.Now()
    time.Sleep(100 * time.Millisecond)
    elapsed := boomer.Now() - start

    /*
    汇报一个成功的结果，实际使用时，根据实际场景，自行判断成功还是失败
    */
    boomer.Events.Publish("request_success", "http", "foo", elapsed, int64(10))
}


func bar(){

    start := boomer.Now()
    time.Sleep(100 * time.Millisecond)
    elapsed := boomer.Now() - start

    /*
    汇报一个失败的结果，实际使用时，根据实际场景，自行判断成功还是失败
    */
    boomer.Events.Publish("request_failure", "udp", "bar", elapsed, "udp error")
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

    // 连接到 master，等待页面上下发指令，支持多个 Task
    boomer.Run(task1, task2)

}
```

## 使用

为了方便调试，可以单独运行 task，不必连接到 master。

```bash
go build -o a.out main.go
./a.out --run-tasks foo,bar
```

限制单个 boomer 实例的最高 RPS(TPS)，在一些指定 RPS(TPS) 的场景下使用。
```bash
go build -o a.out main.go
./a.out --max-rps 10000
```

如果 master 使用 zeromq。

```bash
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
go build -o a.out main.go
./a.out --master-host=127.0.0.1 --master-port=5557 --rpc=zeromq
```

如果 master 使用 TCP Socket。

```bash
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
go build -o a.out main.go
./a.out --master-host=127.0.0.1 --master-port=5557 --rpc=socket
```

locust 启动时，需要一个 locustfile，随便一个符合它要求的即可，这里提供了一个 dummy.py。

由于我们实际上使用 boomer 来施压，这个文件并不会影响到测试。

## License

Open source licensed under the MIT license (see _LICENSE_ file for details).
