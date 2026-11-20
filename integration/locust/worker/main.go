package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/myzhan/boomer"
)

func main() {
	client := &http.Client{Timeout: 2 * time.Second}
	tasks := make([]*boomer.Task, 0, 2)
	for _, path := range []string{"/success", "/failure"} {
		path := path
		tasks = append(tasks, &boomer.Task{Name: path, Weight: 1, Fn: func() {
			defer time.Sleep(20 * time.Millisecond)
			start := time.Now()
			response, err := client.Get(os.Getenv("BOOMER_E2E_TARGET") + path)
			if err != nil {
				boomer.RecordFailure("GET", path, time.Since(start).Milliseconds(), err.Error())
				return
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			elapsed := time.Since(start).Milliseconds()
			if err != nil {
				boomer.RecordFailure("GET", path, elapsed, err.Error())
			} else if response.StatusCode != http.StatusOK {
				boomer.RecordFailure("GET", path, elapsed, "HTTP 503")
			} else {
				boomer.RecordSuccess("GET", path, elapsed, int64(len(body)))
			}
		}})
	}
	boomer.Run(tasks...)
}
