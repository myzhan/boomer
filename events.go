package boomer

import (
	"github.com/asaskevich/EventBus"
)

// Events is core event bus instance of boomer
var Events = EventBus.New()

// RecordSuccess reports a success
func RecordSuccess(requestType, name string, responseTime int64, responseLength int64) {
	defaultStats.requestSuccessChannel <- &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   responseTime,
		responseLength: responseLength,
	}
}

// RecordFailure reports a failure
func RecordFailure(requestType, name string, responseTime int64, exception string) {
	defaultStats.requestFailureChannel <- &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: responseTime,
		error:        exception,
	}
}
