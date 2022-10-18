// This file is kept to ensure backward compatibility.

package boomer

import (
	"flag"
	"fmt"
	"log"
	"math"
	"reflect"
	"sync"
	"time"
)

var masterHost string
var masterPort int
var maxRPS int64
var requestIncreaseRate string
var runTasks string
var memoryProfileFile string
var memoryProfileDuration time.Duration
var cpuProfileFile string
var cpuProfileDuration time.Duration

var successRetiredWarning = &sync.Once{}
var failureRetiredWarning = &sync.Once{}

func createRateLimiter(maxRPS int64, requestIncreaseRate string) (rateLimiter RateLimiter, err error) {
	if requestIncreaseRate != "-1" {
		if maxRPS > 0 {
			log.Println("The max RPS that boomer may generate is limited to", maxRPS, "with a increase rate", requestIncreaseRate)
			rateLimiter, err = NewRampUpRateLimiter(maxRPS, requestIncreaseRate, time.Second)
		} else {
			log.Println("The max RPS that boomer may generate is limited by a increase rate", requestIncreaseRate)
			rateLimiter, err = NewRampUpRateLimiter(math.MaxInt64, requestIncreaseRate, time.Second)
		}
	} else {
		if maxRPS > 0 {
			log.Println("The max RPS that boomer may generate is limited to", maxRPS)
			rateLimiter = NewStableRateLimiter(maxRPS, time.Second)
		}
	}
	return rateLimiter, err
}

// According to locust, responseTime should be int64, in milliseconds.
// But previous version of boomer required responseTime to be float64, so sad.
func convertResponseTime(origin interface{}) int64 {
	responseTime := int64(0)
	if _, ok := origin.(float64); ok {
		responseTime = int64(origin.(float64))
	} else if _, ok := origin.(int64); ok {
		responseTime = origin.(int64)
	} else {
		panic(fmt.Sprintf("responseTime should be float64 or int64, not %s", reflect.TypeOf(origin)))
	}
	return responseTime
}

func legacySuccessHandler(requestType string, name string, responseTime interface{}, responseLength int64) {
	successRetiredWarning.Do(func() {
		log.Println("boomer.Events.Publish(\"request_success\") is less performant and deprecated, use boomer.RecordSuccess() instead.")
	})
	defaultBoomer.RecordSuccess(requestType, name, convertResponseTime(responseTime), responseLength)
}

func legacyFailureHandler(requestType string, name string, responseTime interface{}, exception string) {
	failureRetiredWarning.Do(func() {
		log.Println("boomer.Events.Publish(\"request_failure\") is less performant and deprecated, use boomer.RecordFailure() instead.")
	})
	defaultBoomer.RecordFailure(requestType, name, convertResponseTime(responseTime), exception)
}

func initLegacyEventHandlers() {
	Events.Subscribe("request_success", legacySuccessHandler)
	Events.Subscribe("request_failure", legacyFailureHandler)
}

func init() {
	flag.Int64Var(&maxRPS, "max-rps", 0, "Max RPS that boomer can generate, disabled by default.")
	flag.StringVar(&requestIncreaseRate, "request-increase-rate", "-1", "Request increase rate, disabled by default.")
	flag.StringVar(&runTasks, "run-tasks", "", "Run tasks without connecting to the master, multiply tasks is separated by comma. Usually, it's for debug purpose.")
	flag.StringVar(&masterHost, "master-host", "127.0.0.1", "Host or IP address of locust master for distributed load testing.")
	flag.IntVar(&masterPort, "master-port", 5557, "The port to connect to that is used by the locust master for distributed load testing.")
	flag.StringVar(&memoryProfileFile, "mem-profile", "", "Enable memory profiling.")
	flag.DurationVar(&memoryProfileDuration, "mem-profile-duration", 30*time.Second, "Memory profile duration.")
	flag.StringVar(&cpuProfileFile, "cpu-profile", "", "Enable CPU profiling.")
	flag.DurationVar(&cpuProfileDuration, "cpu-profile-duration", 30*time.Second, "CPU profile duration.")
}
