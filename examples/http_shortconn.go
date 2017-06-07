package main

import "github.com/myzhan/boomer"

import (
	"io"
	"io/ioutil"
	"net/http"
)

func test_http() {

	startTime := boomer.Now()

	tr := &http.Transport{
		DisableKeepAlives: true,
	}

	client := &http.Client{Transport: tr}
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

	task := &boomer.Task{
		Name:   "http_shortconn",
		Weight: 10,
		Fn:     test_http,
	}

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 2000

	boomer.Run(task)

}
