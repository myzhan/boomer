package main

import "github.com/myzhan/boomer"

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func now() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func test_http() {

	startTime := now()

	tr := &http.Transport{
		DisableKeepAlives: true,
	}

	client := &http.Client{Transport: tr}
	resp, err := client.Get("https://localhost/")

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	endTime := now()

	if err != nil {
		boomer.Events.Publish("request_failure", "demo", "http", 0.0, err.Error())
	} else {
		boomer.Events.Publish("request_success", "demo", "http", float64(endTime-startTime), resp.ContentLength)
	}
}

func main() {

	task := &boomer.Task{
		Weight: 10,
		Fn:     test_http,
	}

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 2000

	boomer.Run(task)

}
