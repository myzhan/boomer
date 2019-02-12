package boomer

import "testing"

func TestRecordSuccess(t *testing.T) {
	defaultRunner = newRunner(nil, nil, "asap")
	RecordSuccess("http", "foo", int64(1), int64(10))

	requestSuccessMsg := <-defaultRunner.stats.requestSuccessChannel
	if requestSuccessMsg.requestType != "http" {
		t.Error("Expected: http, got:", requestSuccessMsg.requestType)
	}
	if requestSuccessMsg.responseTime != int64(1) {
		t.Error("Expected: 1, got:", requestSuccessMsg.responseTime)
	}
	defaultRunner = nil
}

func TestRecordFailure(t *testing.T) {
	defaultRunner = newRunner(nil, nil, "asap")
	RecordFailure("udp", "bar", int64(2), "udp error")

	requestFailureMsg := <-defaultRunner.stats.requestFailureChannel
	if requestFailureMsg.requestType != "udp" {
		t.Error("Expected: udp, got:", requestFailureMsg.requestType)
	}
	if requestFailureMsg.responseTime != int64(2) {
		t.Error("Expected: 2, got:", requestFailureMsg.responseTime)
	}
	if requestFailureMsg.error != "udp error" {
		t.Error("Expected: udp error, got:", requestFailureMsg.error)
	}
	defaultRunner = nil
}
