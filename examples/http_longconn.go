package main

import "github.com/myzhan/boomer"

import (
	"io"
	"io/ioutil"
	"net/http"
)

// reuse the same client
var client *http.Client

func test_http() {

	startTime := boomer.Now()

	resp, err := client.Get("https://localhost/")

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	endTime := boomer.Now()

	if err != nil {
		boomer.Events.Publish("request_failure", "http", "demo", 0.0, err.Error())
	} else {
		boomer.Events.Publish("request_success", "http", "demo", float64(endTime-startTime), resp.ContentLength)
	}
}

func main() {

	tr := &http.Transport{
		MaxIdleConnsPerHost: 2000,
	}

	client = &http.Client{Transport: tr}

	task := &boomer.Task{
		Name:   "http_longconn",
		Weight: 10,
		Fn:     test_http,
	}

	boomer.Run(task)

}
