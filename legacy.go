package boomer

import (
	"fmt"
	"log"
	"reflect"
	"sync"
)

var successRetiredWarning = &sync.Once{}
var failureRetiredWarning = &sync.Once{}

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
	defaultRunner.stats.requestSuccessChannel <- &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   convertResponseTime(responseTime),
		responseLength: responseLength,
	}
}

func legacyFailureHandler(requestType string, name string, responseTime interface{}, exception string) {
	failureRetiredWarning.Do(func() {
		log.Println("boomer.Events.Publish(\"request_failure\") is less performant and deprecated, use boomer.RecordFailure() instead.")
	})
	defaultRunner.stats.requestFailureChannel <- &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: convertResponseTime(responseTime),
		error:        exception,
	}
}

func initLegacyEventHandlers() {
	Events.Subscribe("request_success", legacySuccessHandler)
	Events.Subscribe("request_failure", legacyFailureHandler)
}
