package boomer

import (
	"github.com/asaskevich/EventBus"
)

// Events is core event bus instance of boomer
var Events = EventBus.New()

func requestSuccessHandler(requestType string, name string, responseTime float64, responseLength int64) {
	requestSuccessChannel <- &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   responseTime,
		responseLength: responseLength,
	}
}

func requestFailureHandler(requestType string, name string, responseTime float64, exception string) {
	requestFailureChannel <- &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: responseTime,
		error:        exception,
	}
}

func init() {
	Events.Subscribe("request_success", requestSuccessHandler)
	Events.Subscribe("request_failure", requestFailureHandler)
}
