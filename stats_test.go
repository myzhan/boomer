package boomer

import (
	"testing"
	"time"
)

func TestLogRequest(t *testing.T) {
	newStats := newRequestStats()
	newStats.logRequest("http", "success", 2, 30)
	newStats.logRequest("http", "success", 3, 40)
	newStats.logRequest("http", "success", 2, 40)
	newStats.logRequest("http", "success", 1, 20)
	entry := newStats.get("success", "http")

	if entry.numRequests != 4 {
		t.Error("numRequests is wrong, expected: 4, got:", entry.numRequests)
	}
	if entry.minResponseTime != 1 {
		t.Error("minResponseTime is wrong, expected: 1, got:", entry.minResponseTime)
	}
	if entry.maxResponseTime != 3 {
		t.Error("maxResponseTime is wrong, expected: 3, got:", entry.maxResponseTime)
	}
	if entry.totalResponseTime != 8 {
		t.Error("totalResponseTime is wrong, expected: 8, got:", entry.totalResponseTime)
	}
	if entry.totalContentLength != 130 {
		t.Error("totalContentLength is wrong, expected: 130, got:", entry.totalContentLength)
	}

	// check newStats.total
	if newStats.total.numRequests != 4 {
		t.Error("newStats.total.numRequests is wrong, expected: 4, got:", newStats.total.numRequests)
	}
	if newStats.total.minResponseTime != 1 {
		t.Error("newStats.total.minResponseTime is wrong, expected: 1, got:", newStats.total.minResponseTime)
	}
	if newStats.total.maxResponseTime != 3 {
		t.Error("newStats.total.maxResponseTime is wrong, expected: 3, got:", newStats.total.maxResponseTime)
	}
	if newStats.total.totalResponseTime != 8 {
		t.Error("newStats.total.totalResponseTime is wrong, expected: 8, got:", newStats.total.totalResponseTime)
	}
	if newStats.total.totalContentLength != 130 {
		t.Error("newStats.total.totalContentLength is wrong, expected: 130, got:", newStats.total.totalContentLength)
	}
}

func BenchmarkLogRequest(b *testing.B) {
	newStats := newRequestStats()
	for i := 0; i < b.N; i++ {
		newStats.logRequest("http", "success", 2, 30)
	}
}

func TestRoundedResponseTime(t *testing.T) {
	newStats := newRequestStats()
	newStats.logRequest("http", "success", 147, 1)
	newStats.logRequest("http", "success", 3432, 1)
	newStats.logRequest("http", "success", 58760, 1)
	entry := newStats.get("success", "http")
	responseTimes := entry.responseTimes

	if len(responseTimes) != 3 {
		t.Error("len(responseTimes) is wrong, expected: 3, got:", len(responseTimes))
	}

	if val, ok := responseTimes[150]; !ok || val != 1 {
		t.Error("Rounded response time should be", 150)
	}

	if val, ok := responseTimes[3400]; !ok || val != 1 {
		t.Error("Rounded response time should be", 3400)
	}

	if val, ok := responseTimes[59000]; !ok || val != 1 {
		t.Error("Rounded response time should be", 59000)
	}
}

func TestLogError(t *testing.T) {
	newStats := newRequestStats()
	newStats.logError("http", "failure", "500 error")
	newStats.logError("http", "failure", "400 error")
	newStats.logError("http", "failure", "400 error")
	entry := newStats.get("failure", "http")

	if entry.numFailures != 3 {
		t.Error("numFailures is wrong, expected: 3, got:", entry.numFailures)
	}

	if newStats.total.numFailures != 3 {
		t.Error("newStats.total.numFailures is wrong, expected: 3, got:", newStats.total.numFailures)
	}

	// md5("httpfailure500 error") = 547c38e4e4742c1c581f9e2809ba4f55
	err500 := newStats.errors["547c38e4e4742c1c581f9e2809ba4f55"]
	if err500.error != "500 error" {
		t.Error("Error message is wrong, expected: 500 error, got:", err500.error)
	}
	if err500.occurences != 1 {
		t.Error("Error occurences is wrong, expected: 1, got:", err500.occurences)
	}

	// md5("httpfailure400 error") = f391c310401ad8e10e929f2ee1a614e4
	err400 := newStats.errors["f391c310401ad8e10e929f2ee1a614e4"]
	if err400.error != "400 error" {
		t.Error("Error message is wrong, expected: 400 error, got:", err400.error)
	}
	if err400.occurences != 2 {
		t.Error("Error occurences is wrong, expected: 2, got:", err400.occurences)
	}

}

func BenchmarkLogError(b *testing.B) {
	newStats := newRequestStats()
	for i := 0; i < b.N; i++ {
		// LogError use md5 to calculate hash keys, it may slow down the only goroutine,
		// which consumes both requestSuccessChannel and requestFailureChannel.
		newStats.logError("http", "failure", "500 error")
	}
}

func TestClearAll(t *testing.T) {
	newStats := newRequestStats()
	newStats.logRequest("http", "success", 1, 20)
	newStats.clearAll()

	if newStats.total.numRequests != 0 {
		t.Error("After clearAll(), newStats.total.numRequests is wrong, expected: 0, got:", newStats.total.numRequests)
	}
}

func TestClearAllByChannel(t *testing.T) {
	newStats := newRequestStats()
	newStats.start()
	defer newStats.close()
	newStats.logRequest("http", "success", 1, 20)
	newStats.clearStatsChannel <- true

	if newStats.total.numRequests != 0 {
		t.Error("After clearAll(), newStats.total.numRequests is wrong, expected: 0, got:", newStats.total.numRequests)
	}
}

func TestSerializeStats(t *testing.T) {
	newStats := newRequestStats()
	newStats.logRequest("http", "success", 1, 20)

	serialized := newStats.serializeStats()
	if len(serialized) != 1 {
		t.Error("The length of serialized results is wrong, expected: 1, got:", len(serialized))
		return
	}

	first := serialized[0].(map[string]interface{})
	if first["name"].(string) != "success" {
		t.Error("The name is wrong, expected:", "success", "got:", first["name"].(string))
	}
	if first["method"].(string) != "http" {
		t.Error("The method is wrong, expected:", "http", "got:", first["method"].(string))
	}
	if first["num_requests"].(int64) != int64(1) {
		t.Error("The num_requests is wrong, expected:", 1, "got:", first["num_requests"].(int64))
	}
	if first["num_failures"].(int64) != int64(0) {
		t.Error("The num_failures is wrong, expected:", 0, "got:", first["num_failures"].(int64))
	}
}

func TestSerializeErrors(t *testing.T) {
	newStats := newRequestStats()
	newStats.logError("http", "failure", "500 error")
	newStats.logError("http", "failure", "400 error")
	newStats.logError("http", "failure", "400 error")
	serialized := newStats.serializeErrors()

	if len(serialized) != 2 {
		t.Error("The length of serialized results is wrong, expected: 2, got:", len(serialized))
		return
	}

	for key, value := range serialized {
		if key == "f391c310401ad8e10e929f2ee1a614e4" {
			err := value["error"].(string)
			if err != "400 error" {
				t.Error("expected: 400 error, got:", err)
			}
			occurences := value["occurences"].(int64)
			if occurences != int64(2) {
				t.Error("expected: 2, got:", occurences)
			}
		}
	}
}

func TestCollectReportData(t *testing.T) {
	newStats := newRequestStats()
	newStats.logRequest("http", "success", 2, 30)
	newStats.logError("http", "failure", "500 error")
	result := newStats.collectReportData()

	if _, ok := result["stats"]; !ok {
		t.Error("Key stats not found")
	}
	if _, ok := result["stats_total"]; !ok {
		t.Error("Key stats not found")
	}
	if _, ok := result["errors"]; !ok {
		t.Error("Key stats not found")
	}
}

func TestStatsStart(t *testing.T) {
	newStats := newRequestStats()
	newStats.start()
	defer newStats.close()

	newStats.requestSuccessChannel <- &requestSuccess{
		requestType:    "http",
		name:           "success",
		responseTime:   2,
		responseLength: 30,
	}

	newStats.requestFailureChannel <- &requestFailure{
		requestType:  "http",
		name:         "failure",
		responseTime: 1,
		error:        "500 error",
	}

	var ticker = time.NewTicker(slaveReportInterval + 100*time.Millisecond)
	for {
		select {
		case <-ticker.C:
			t.Error("Timeout waiting for stats reports to runner")
		case <-newStats.messageToRunner:
			goto end
		}
	}
end:
}
