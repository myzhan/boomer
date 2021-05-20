package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"github.com/myzhan/boomer"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Config and urls in json file.
// Run in task set.

var client *http.Client
var targetFile string

var _post []byte
var err1 interface{}

type Target struct {
	Method      string `json:"method"`
	Url         string `json:"url"`
	PostFile    string `json:"postFile"`
	ContentType string `json:"contentType"`
	Verbose     bool   `json:"verbose"`
	Weight      int    `json:"weight"`
	Name        string `json:"name"`
}

type TargetF struct {
	Config  TargetConfig `json:"config"`
	Targets []Target     `json:"targets"`
}
type TargetConfig struct {
	Timeout            int  `json:"timeout"`
	DisableCompression bool `json:"disableCompression"`
	DisableKeepalive   bool `json:"disableKeepalive"`
}

func (t *Target) worker() {
	if t.PostFile == "GET" || t.Method == "DELETE" || t.PostFile == "" {
		_post = []byte(nil)
	} else {
		_post, err1 = ioutil.ReadFile(t.PostFile)
		if err1 != nil {
			log.Fatalf("ERROR: load post file error: %s", err1)
		}
	}
	request, err := http.NewRequest(t.Method, t.Url, bytes.NewBuffer(_post))
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	request.Header.Set("Content-Type", t.ContentType)

	startTime := time.Now()
	response, err := client.Do(request)
	elapsed := time.Since(startTime)

	if err != nil {
		if t.Verbose {
			log.Printf("%v\n", err)
		}
		boomer.RecordFailure(t.Method, t.Url, 0.0, err.Error())
	} else {
		boomer.RecordSuccess(t.Method, t.Url,
			elapsed.Nanoseconds()/int64(time.Millisecond), response.ContentLength)

		if t.Verbose {
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

func main() {
	flag.StringVar(&targetFile, "f", "", "target file in json")
	flag.Parse()
	if targetFile == "" {
		log.Fatalln("-f can't be empty string, please specify a json file that you want to test.")
	}

	targetDate, err := ioutil.ReadFile(targetFile)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	var t TargetF
	errs := json.Unmarshal(targetDate, &t)
	if errs != nil {
		log.Fatalln("===Error:", errs)
	}
	config := t.Config
	timeout := config.Timeout
	disableCompression := config.DisableCompression
	disableKeepalive := config.DisableKeepalive
	log.Printf(`HTTP benchmark Config:
		timeout: %d
		disable-compression: %t
		disable-keepalive: %t`, timeout, disableCompression, disableKeepalive)

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
	//tasks := make([]*boomer.Task, 0)
	ts := boomer.NewWeighingTaskSet()
	targets := t.Targets
	for num, _t := range targets {
		method := _t.Method
		url := _t.Url
		contentType := _t.ContentType
		verbose := _t.Verbose
		weight := _t.Weight
		name := _t.Name
		postFile := _t.PostFile
		log.Printf(`HTTP benchmark Target-%d:
		method: %s
		url: %s
		content-type: %s
		verbose: %t`, num, method, url, contentType, verbose)

		_target := Target{
			Method:      method,
			Url:         url,
			PostFile:    postFile,
			ContentType: contentType,
			Verbose:     verbose,
			Weight:      weight,
			Name:        name,
		}
		_task := &boomer.Task{
			Name:   _target.Name,
			Weight: _target.Weight,
			Fn:     _target.worker,
		}
		//tasks = append(tasks, _task)
		ts.AddTask(_task)
	}
	tasks := &boomer.Task{
		Name: "TaskSet",
		Fn:   ts.Run,
	}
	boomer.Run(tasks)
}
