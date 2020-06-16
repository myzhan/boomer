package main

import (
	"fmt"
	"io"
	"io/ioutil"

	//"io"
	//"io/ioutil"

	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/myzhan/boomer"
	"github.com/valyala/fasthttp"
)

func createHttpClient() *http.Client{
	client := &http.Client{
		Transport : &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			DisableKeepAlives: false,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10000,
			MaxIdleConnsPerHost:   10000,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	return client
}

var client *http.Client = createHttpClient()

func foo() {
	start := time.Now()

	var url string = `http://localhost:8080/hello`
	resp, err := client.Get(url)
	if err != nil {
		//log.Println(err)
		return
	}

	//all Write calls succeed without doing anything.
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	//fmt.Println(resp.Status)
	elapsed := time.Since(start)

	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	if resp.StatusCode < 400  {
		globalBoomer.RecordSuccess("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), int64(10))
	} else {
		globalBoomer.RecordFailure("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), resp.Status)
	}

}

//use request and response pool
func foo1(){
	start := time.Now()

	var req = fasthttp.AcquireRequest()
	var resp = fasthttp.AcquireResponse()
	var url string = `http://localhost:8080/hello`
	req.Header.SetMethod("GET")
	req.SetRequestURI(url)
	defer func(){
		fasthttp.ReleaseResponse(resp)
		fasthttp.ReleaseRequest(req)
	}()
	//client := createHttpClient()

	//statusCode, body, err := client.Get(body,"http://localhost:8080/hello")
	//resp, err := client.Get("http://localhost:80/hello")

	if err := fasthttp.Do(req, resp);err != nil {
		//log.Println(err)
		return
	}

	//resp.ReleaseBody(0)
	//all Write calls succeed without doing anything.
	//io.Copy(ioutil.Discard, resp.body)
	//resp.body.Close()

	//fmt.Println(resp.Status)
	elapsed := time.Since(start)

	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	if resp.StatusCode() < 400  {
		globalBoomer.RecordSuccess("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), int64(10))
	} else {
		globalBoomer.RecordFailure("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), fmt.Sprint(resp.StatusCode()))
	}
}

//use fasthttp.Client
var fast_client *fasthttp.Client = &fasthttp.Client{
	MaxConnsPerHost: 10000,
}

func foo2(){
	start := time.Now()


	var url string = `http://localhost:8080/hello`


	statusCode, _, err := fast_client.Get(nil,url)
	if err != nil {
		//log.Println(err)
		return
	}

	//resp.ReleaseBody(0)
	//all Write calls succeed without doing anything.
	//io.Copy(ioutil.Discard, resp.body)
	//resp.body.Close()

	//fmt.Println(resp.Status)
	elapsed := time.Since(start)

	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	if statusCode < 400  {
		globalBoomer.RecordSuccess("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), int64(10))
	} else {
		globalBoomer.RecordFailure("http", "foo", elapsed.Nanoseconds()/int64(time.Millisecond), fmt.Sprint(statusCode))
	}
}


func bar() {
	start := time.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := time.Since(start)

	// Report your test result as a failure, if you write it in python, it will looks like this
	// events.request_failure.fire(request_type="udp", name="bar", response_time=100, exception=Exception("udp error"))
	globalBoomer.RecordFailure("udp", "bar", elapsed.Nanoseconds()/int64(time.Millisecond), "udp error")
}

func waitForQuit() {
	wg := sync.WaitGroup{}
	wg.Add(1)

	quitByMe := false
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		quitByMe = true
		globalBoomer.Quit()
		wg.Done()
	}()

	boomer.Events.Subscribe("boomer:quit", func() {
		if !quitByMe {
			wg.Done()
		}
	})

	wg.Wait()
}

var globalBoomer = boomer.NewBoomer("127.0.0.1", 5557)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	task1 := &boomer.Task{
		Name:   "foo",
		// The weight is used to distribute goroutines over multiple tasks.
		Weight: 10,
		Fn:     foo2,
	}

	//task2 := &boomer.Task{
	//	Name:   "bar",
	//	Weight: 30,
	//	Fn:     bar,
	//}

	//globalBoomer.Run(task1, task2)
	globalBoomer.Run(task1)

	waitForQuit()
	log.Println("shut down")

}




