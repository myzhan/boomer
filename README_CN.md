# boomer

## boomer是什么？

boomer 完整地实现了 locust 的通讯协议，运行在 slave 模式下，用 goroutine 来执行用户提供的测试函数，然后将测试结果上报给运行在 master 模式下的 locust。

与 locust 原生的实现相比，解决了两个问题。一是单台施压机上，能充分利用多个 CPU 核心来施压，二是再也不用提防阻塞 IO 操作导致 gevent 阻塞。

## 安装

```bash
go get github.com/myzhan/boomer
```

### 使用zeromq（可选）
安装 [goczmq](https://github.com/zeromq/goczmq#building-from-source-linux) 依赖后，可以使用 zeromq 与 master 进行通信，获得更好的通讯性能。

默认的 --rpc 参数值是 zeromq，启动时，可以使用 --rpc=socket 切换到普通的 TCP Socket。

### 不使用zeromq
如果想快速使用，也可以不安装 zeromq 相关依赖，直接使用 TCP Socket 来和 master 通讯。

**无论选择了哪个，locust(master) 和 boomer 要使用一致的协议，才能互通。**

## 例子(main.go)
下面演示一下 boomer 的 API，可以在 examples 目录下找到更多的例子。

```go
package main


import "github.com/myzhan/boomer"
import "time"


func foo(){
    time.Sleep(100 * time.Millisecond)
    /*
    汇报一个成功的结果，实际使用时，根据实际场景，自行判断成功还是失败
    */
    boomer.Events.Publish("request_success", "foo", "http", 100.0, int64(10))
}


func bar(){
    time.Sleep(100 * time.Millisecond)
    /*
    汇报一个失败的结果，实际使用时，根据实际场景，自行判断成功还是失败
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

如果 master 使用 zeromq。

```bash
# 启动一个 master，依然是 locust
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
# 构建可执行文件
go build -tags 'zeromq' -o a.out main.go
# 连接 master，使用 zeromq
./a.out --master-host=127.0.0.1 --master-port=5557 --rpc=zeromq
```

如果 master 使用 TCP Socket。

```bash
# 启动一个 master，依然是 locust
locust -f dummy.py --master --master-bind-host=127.0.0.1 --master-bind-port=5557
# 构建可执行文件
go build -o a.out main.go
# 连接 master，使用 TCP Socket
./a.out --master-host=127.0.0.1 --master-port=5557
```

locust 启动时，需要一个 locustfile，随便一个符合它要求的即可，这里提供了一个 dummy.py。

由于我们实际上使用 boomer 来施压，这个文件并不会影响到测试。

## License

Open source licensed under the MIT license (see _LICENSE_ file for details).
