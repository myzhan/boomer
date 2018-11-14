package boomer

import (
	"testing"
)

func TestConvertResponseTime(t *testing.T) {
	convertedFloat := convertResponseTime(float64(1.234))
	convertedInt64 := convertResponseTime(int64(2))
	if convertedFloat != 1 {
		t.Error("Failed to convert responseTime from float64")
	}
	if convertedInt64 != 2 {
		t.Error("Failed to convert responseTime from int64")
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("It should panic")
		}
	}()
	// It should panic
	convertResponseTime(1)
}

func TestInitEvents(t *testing.T) {
	initEvents()
	Events.Publish("request_success", "http", "foo", int64(1), int64(10))
	Events.Publish("request_failure", "udp", "bar", int64(2), "udp error")

	requestSuccessMsg := <-requestSuccessChannel
	if requestSuccessMsg.requestType != "http" {
		t.Error("Expected: http, got:", requestSuccessMsg.requestType)
	}
	if requestSuccessMsg.responseTime != int64(1) {
		t.Error("Expected: 1, got:", requestSuccessMsg.responseTime)
	}

	requestFailureMsg := <-requestFailureChannel
	if requestFailureMsg.requestType != "udp" {
		t.Error("Expected: udp, got:", requestFailureMsg.requestType)
	}
	if requestFailureMsg.responseTime != int64(2) {
		t.Error("Expected: 2, got:", requestFailureMsg.responseTime)
	}
	if requestFailureMsg.error != "udp error" {
		t.Error("Expected: udp error, got:", requestFailureMsg.error)
	}

	Events.Unsubscribe("request_success", requestSuccessHandler)
	Events.Unsubscribe("request_failure", requestFailureHandler)
}
