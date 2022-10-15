package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/myzhan/boomer"
)

// This is a tool like Apache Benchmark a.k.a "ab".
// It doesn't implement all the features supported by ab.

var client *http.Client
var postBody []byte

var verbose bool

// this is just a demo for showing how to construct multiple boomer tasks
// and conduct testing related to the requests of http://httpbin.org
// user can also set the post body by themselves
var host = "http://httpbin.org"
var timeout int
var postFile string
var contentType string

var disableCompression bool
var disableKeepalive bool

var postHttpbinWeight int
var getHttpbinWeight int

var locustMasterHost string
var locustMasterPort int
var globalBoomer *boomer.Boomer

func postHttpbin() {
	postHttpbinUrl := fmt.Sprintf("%s/post", host)
	requestAndRecord("POST", postHttpbinUrl, bytes.NewBuffer(postBody))
}

func getHttpbin() {
	getHttpbinUrl := fmt.Sprintf("%s/get", host)
	requestAndRecord("GET", getHttpbinUrl, nil)
}

func requestAndRecord(method, reqUrl string, bodyReader io.Reader) {
	request, err := http.NewRequest(method, reqUrl, bodyReader)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	request.Header.Set("Content-Type", contentType)

	startTime := time.Now()
	response, err := client.Do(request)
	elapsed := time.Since(startTime)

	if err != nil {
		if verbose {
			log.Printf("%v\n", err)
		}
		globalBoomer.RecordFailure("http", reqUrl, 0.0, err.Error())
	} else {
		globalBoomer.RecordSuccess("http", reqUrl,
			elapsed.Nanoseconds()/int64(time.Millisecond), response.ContentLength)

		if verbose {
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Printf("%v\n", err)
			} else {
				log.Printf("Status Code: %d\n", response.StatusCode)
				log.Println(string(body))
			}

		} else {
			io.Copy(ioutil.Discard, response.Body)
		}

		response.Body.Close()
	}
}

func waitForQuit() {
	wg := sync.WaitGroup{}
	wg.Add(1)

	quitByMe := false
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		quitByMe = true
		globalBoomer.Quit()
		wg.Done()
	}()

	boomer.Events.Subscribe(boomer.EVENT_QUIT, func() {
		if !quitByMe {
			wg.Done()
		}
	})

	wg.Wait()
}

func testHttpbinServer() {
	testRequestFn := func(method, reqUrl string, bodyReader io.Reader) {
		request, err := http.NewRequest(method, reqUrl, bodyReader)
		if err != nil {
			log.Fatalf("%v\n", err)
		}

		request.Header.Set("Content-Type", contentType)
		response, err := client.Do(request)
		if err != nil {
			log.Fatalf("%v\n", err)
		}
		if response.StatusCode != http.StatusOK {
			log.Fatalf("could not get status ok from the server with host: %s\n", reqUrl)
		}
	}
	testRequestFn("GET", fmt.Sprintf("%s/get", host), nil)
	testRequestFn("POST", fmt.Sprintf("%s/post", host), bytes.NewBuffer(postBody))
}

func main() {

	flag.IntVar(&timeout, "timeout", 10, "Seconds to max. wait for each response")
	flag.StringVar(&postFile, "post-file", "", "File containing data to POST. Remember also to set --content-type")
	flag.StringVar(&contentType, "content-type", "text/plain", "Content-type header")

	flag.StringVar(&locustMasterHost, "locust-master-host", "127.0.0.1", "locust master host")
	flag.IntVar(&locustMasterPort, "locust-master-port", 5557, "locust master port")

	flag.BoolVar(&disableCompression, "disable-compression", false, "Disable compression")
	flag.BoolVar(&disableKeepalive, "disable-keepalive", false, "Disable keepalive")

	flag.BoolVar(&verbose, "verbose", false, "Print debug log")

	flag.IntVar(&postHttpbinWeight, "post-httpbin-weight", 10, "set weight when use httpbin post request")
	flag.IntVar(&getHttpbinWeight, "get-httpbin-weight", 10, "set weight when use httpbin get request")
	flag.Parse()

	log.Printf(`HTTP benchmark is running with these args:
host: %s
timeout: %d
post-file: %s
content-type: %s
disable-compression: %t
disable-keepalive: %t
verbose: %t
locust-master-host: %s
locust-master-port: %d
post-httpbin-weight: %d
get-httpbin-weight: %d`,
		host, timeout, postFile, contentType, disableCompression, disableKeepalive, verbose,
		locustMasterHost, locustMasterPort, postHttpbinWeight, getHttpbinWeight)

	if postFile != "" {
		tmp, err := ioutil.ReadFile(postFile)
		if err != nil {
			log.Fatalf("%v\n", err)
		}
		postBody = tmp
	}
	globalBoomer = boomer.NewBoomer(locustMasterHost, locustMasterPort)
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 2000
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConnsPerHost: 2000,
		DisableCompression:  disableCompression,
		DisableKeepAlives:   disableKeepalive,
	}
	client = &http.Client{
		Transport: tr,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	testHttpbinServer()
	task1 := &boomer.Task{
		Name:   "post-httpbin",
		Weight: postHttpbinWeight,
		Fn:     postHttpbin,
	}
	task2 := &boomer.Task{
		Name:   "get-httpbin",
		Weight: getHttpbinWeight,
		Fn:     getHttpbin,
	}
	globalBoomer.Run(task1, task2)

	waitForQuit()
	log.Println("test finished")
}
