package boomer

import (
	"fmt"
	"reflect"

	"github.com/asaskevich/EventBus"
)

// Events is core event bus instance of boomer
var Events = EventBus.New()

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

func requestSuccessHandler(requestType string, name string, responseTime interface{}, responseLength int64) {
	defaultStats.requestSuccessChannel <- &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   convertResponseTime(responseTime),
		responseLength: responseLength,
	}
}

func requestFailureHandler(requestType string, name string, responseTime interface{}, exception string) {
	defaultStats.requestFailureChannel <- &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: convertResponseTime(responseTime),
		error:        exception,
	}
}

func initEvents() {
	Events.Subscribe("request_success", requestSuccessHandler)
	Events.Subscribe("request_failure", requestFailureHandler)
}
