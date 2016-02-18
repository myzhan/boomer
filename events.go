package boomer


import (
	"github.com/asaskevich/EventBus"
)


var Events = EventBus.New()


func requestSuccessHandler(requestType string, name string, responseTime float64, responseLength int64) {
	RequestSuccessChannel <- &RequestSuccess{
		requestType: requestType,
		name: name,
		responseTime: responseTime,
		responseLength: responseLength,
	}
}


func requestFailureHandler(requestType string, name string, responseTime float64, exception string) {
	RequestFailureChannel <- &RequestFailure{
		requestType: requestType,
		name: name,
		responseTime: responseTime,
		error: exception,
	}
}


func init(){
	Events.Subscribe("request_success", requestSuccessHandler)
	Events.Subscribe("request_failure", requestFailureHandler)
}