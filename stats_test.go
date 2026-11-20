package boomer

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func BenchmarkLogRequest(b *testing.B) {
	newStats := newRequestStats()
	for i := 0; i < b.N; i++ {
		newStats.logRequest("http", "success", 2, 30)
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

var _ = Describe("Test states", func() {

	It("test log request", func() {
		newStats := newRequestStats()
		newStats.logRequest("http", "success", 2, 30)
		newStats.logRequest("http", "success", 3, 40)
		newStats.logRequest("http", "success", 2, 40)
		newStats.logRequest("http", "success", 1, 20)
		entry := newStats.get("success", "http")

		Expect(entry.NumRequests).To(BeEquivalentTo(4))
		Expect(entry.MinResponseTime).To(BeEquivalentTo(1))
		Expect(entry.MaxResponseTime).To(BeEquivalentTo(3))
		Expect(entry.TotalResponseTime).To(BeEquivalentTo(8))
		Expect(entry.TotalContentLength).To(BeEquivalentTo(130))

		Expect(newStats.total.NumRequests).To(BeEquivalentTo(4))
		Expect(newStats.total.MinResponseTime).To(BeEquivalentTo(1))
		Expect(newStats.total.MaxResponseTime).To(BeEquivalentTo(3))
		Expect(newStats.total.TotalResponseTime).To(BeEquivalentTo(8))
		Expect(newStats.total.TotalContentLength).To(BeEquivalentTo(130))
	})

	It("test rounded response time", func() {
		newStats := newRequestStats()
		newStats.logRequest("http", "success", 147, 1)
		newStats.logRequest("http", "success", 3432, 1)
		newStats.logRequest("http", "success", 58760, 1)
		entry := newStats.get("success", "http")
		responseTimes := entry.ResponseTimes

		Expect(responseTimes).To(HaveLen(3))
		Expect(responseTimes).To(HaveKeyWithValue(int64(150), int64(1)))
		Expect(responseTimes).To(HaveKeyWithValue(int64(3400), int64(1)))
		Expect(responseTimes).To(HaveKeyWithValue(int64(59000), int64(1)))
	})

	It("test log error", func() {
		newStats := newRequestStats()
		newStats.logError("http", "failure", "500 error")
		newStats.logError("http", "failure", "400 error")
		newStats.logError("http", "failure", "400 error")
		entry := newStats.get("failure", "http")

		Expect(entry.NumFailures).To(BeEquivalentTo(3))
		Expect(newStats.total.NumFailures).To(BeEquivalentTo(3))

		// md5("httpfailure500 error") = 547c38e4e4742c1c581f9e2809ba4f55
		err500 := newStats.errors["547c38e4e4742c1c581f9e2809ba4f55"]
		Expect(err500.error).To(Equal("500 error"))
		Expect(err500.occurrences).To(BeEquivalentTo(1))

		err400 := newStats.errors["f391c310401ad8e10e929f2ee1a614e4"]
		Expect(err400.error).To(Equal("400 error"))
		Expect(err400.occurrences).To(BeEquivalentTo(2))
	})

	It("test clear all", func() {
		newStats := newRequestStats()
		newStats.logRequest("http", "success", 1, 20)
		newStats.clearAll()
		Expect(newStats.total.NumRequests).To(BeEquivalentTo(0))
	})

	It("test clear all by channel", func() {
		newStats := newRequestStats()
		newStats.start()
		defer newStats.close()
		newStats.requestSuccessChan <- &requestSuccess{
			requestType:    "http",
			name:           "success",
			responseTime:   1,
			responseLength: 20,
		}
		// Wait for the first report to ensure the success is processed
		report := <-newStats.messageToRunnerChan
		stats := report["stats"].([]interface{})
		Expect(len(stats)).To(BeEquivalentTo(1))
		// Now clear and verify it takes effect in the next report
		newStats.clearStatsChan <- true
		report = <-newStats.messageToRunnerChan
		stats = report["stats"].([]interface{})
		Expect(len(stats)).To(BeEquivalentTo(0))
	})

	It("test serialize stats", func() {
		newStats := newRequestStats()
		newStats.logRequest("http", "success", 1, 20)

		serialized := newStats.serializeStats()
		Expect(serialized).To(HaveLen(1))

		first := serialized[0]
		entry, err := deserializeStatsEntry(first)
		Expect(err).NotTo(HaveOccurred())

		Expect(entry.Name).To(Equal("success"))
		Expect(entry.Method).To(Equal("http"))
		Expect(entry.NumRequests).To(BeEquivalentTo(1))
		Expect(entry.NumFailures).To(BeEquivalentTo(0))
	})

	It("test serialize errors", func() {
		newStats := newRequestStats()
		newStats.logError("http", "failure", "500 error")
		newStats.logError("http", "failure", "400 error")
		newStats.logError("http", "failure", "400 error")
		serialized := newStats.serializeErrors()

		Expect(serialized).To(HaveLen(2))
		Expect(serialized).To(HaveKeyWithValue("f391c310401ad8e10e929f2ee1a614e4", map[string]interface{}{
			"error":       "400 error",
			"occurrences": int64(2),
			"name":        "failure",
			"method":      "http",
		}))
	})

	It("test stats start", func() {
		newStats := newRequestStats()
		newStats.start()
		defer newStats.close()

		newStats.requestSuccessChan <- &requestSuccess{
			requestType:    "http",
			name:           "success",
			responseTime:   2,
			responseLength: 30,
		}

		newStats.requestFailureChan <- &requestFailure{
			requestType:  "http",
			name:         "failure",
			responseTime: 1,
			error:        "500 error",
		}

		Eventually(newStats.messageToRunnerChan).WithTimeout(slaveReportInterval + 500*time.Millisecond).Should(Receive())
	})
})

func TestRequestStatsReportKeepsActiveEntries(t *testing.T) {
	stats := newRequestStats()
	key := entryKey{name: "active", method: "GET"}
	first := stats.get(key.name, key.method)
	for i := 0; i < idleEntryTTL*2; i++ {
		stats.logRequest("GET", "active", 1, 1)
		stats.collectReportData()
		if stats.entries[key] != first {
			t.Fatal("report evicted a continuously active entry")
		}
	}
	for i := 0; i < idleEntryTTL-1; i++ {
		stats.collectReportData()
	}
	if stats.entries[key] != first {
		t.Fatal("entry evicted before the idle TTL elapsed")
	}
	stats.collectReportData()
	if _, ok := stats.entries[key]; ok {
		t.Fatal("idle entry retained after the TTL elapsed")
	}
}

func TestRequestStatsCloseIsIdempotent(t *testing.T) {
	stats := newRequestStats()
	stats.start()
	stats.close()
	// Closing twice used to panic with "close of closed channel".
	stats.close()
}

// The goroutine of requestStats must stop consuming records once close is called.
func TestRequestStatsStopsAfterClose(t *testing.T) {
	stats := newRequestStats()
	stats.start()
	stats.close()

	// Give the goroutine a chance to observe the closed shutdown channel.
	time.Sleep(100 * time.Millisecond)

	stats.requestSuccessChan <- &requestSuccess{
		requestType:    "http",
		name:           "after-close",
		responseTime:   1,
		responseLength: 1,
	}

	time.Sleep(100 * time.Millisecond)
	if left := len(stats.requestSuccessChan); left != 1 {
		t.Fatalf("stats goroutine is still consuming records after close, %d left", left)
	}
}

// Records which were still queued when close is called can't be reported
// anymore, and they must not occupy the buffer either.
func TestRequestStatsCloseDrainsQueue(t *testing.T) {
	stats := newRequestStats()
	for i := 0; i < cap(stats.requestSuccessChan); i++ {
		stats.requestSuccessChan <- &requestSuccess{name: "queued"}
	}
	stats.requestFailureChan <- &requestFailure{name: "queued"}

	stats.close()

	if n := len(stats.requestSuccessChan); n != 0 {
		t.Fatalf("close left %d success records in the queue", n)
	}
	if n := len(stats.requestFailureChan); n != 0 {
		t.Fatalf("close left %d failure records in the queue", n)
	}
}

// Concatenating name and method used to make ("ab", "c") and ("a", "bc") collide.
func TestRequestStatsEntriesDoNotCollide(t *testing.T) {
	stats := newRequestStats()
	first := stats.get("ab", "c")
	second := stats.get("a", "bc")
	if first == second {
		t.Fatal("entries with different name and method must not collide")
	}
	if first.Name != "ab" || first.Method != "c" {
		t.Fatalf("unexpected entry, got name=%q method=%q", first.Name, first.Method)
	}
	if second.Name != "a" || second.Method != "bc" {
		t.Fatalf("unexpected entry, got name=%q method=%q", second.Name, second.Method)
	}

	// Both pairs must keep their own records, and lookups must be stable.
	stats.logRequest("c", "ab", 1, 1)
	stats.logRequest("bc", "a", 1, 1)
	stats.logRequest("c", "ab", 1, 1)
	if stats.get("ab", "c") != first || stats.get("a", "bc") != second {
		t.Fatal("get must keep returning the entry of each pair")
	}
	if first.NumRequests != 2 || second.NumRequests != 1 {
		t.Fatalf("records leaked between colliding entries, got %d and %d", first.NumRequests, second.NumRequests)
	}
}

// Idle entries are dropped once they reach the TTL, and a dropped pair gets a
// fresh entry on its next request. Entries which received requests must never be
// dropped.
func TestRequestStatsDropsIdleEntries(t *testing.T) {
	stats := newRequestStats()
	idleKey := entryKey{name: "idle", method: "GET"}
	activeKey := entryKey{name: "active", method: "GET"}
	idle := stats.get("idle", "GET")
	active := stats.get("active", "GET")
	stats.logRequest("GET", "active", 1, 1) // keep the second pair alive

	for i := 0; i < idleEntryTTL-1; i++ {
		stats.dropIdleEntries()
	}
	if _, ok := stats.entries[idleKey]; !ok {
		t.Fatal("idle entry should not be dropped before reaching the TTL")
	}

	stats.dropIdleEntries()
	if _, ok := stats.entries[idleKey]; ok {
		t.Fatal("idle entry should be dropped after reaching the TTL")
	}
	if _, ok := stats.entries[activeKey]; !ok {
		t.Fatal("the active entry must be kept")
	}

	// The dropped pair comes back with a fresh entry, next to the active one.
	if revived := stats.get("idle", "GET"); revived == idle || revived == active {
		t.Fatal("the revived pair must get a fresh entry")
	}

	// The active entry stays, no matter how many periods pass.
	for i := 0; i < idleEntryTTL*2; i++ {
		stats.dropIdleEntries()
	}
	if stats.get("active", "GET") != active {
		t.Fatal("the active entry must be kept and reused")
	}
	if active.NumRequests != 1 {
		t.Fatalf("the active entry was modified, got %d requests", active.NumRequests)
	}
}

// The field names below are taken from locust's StatsEntryDict and StatsErrorDict,
// renaming or dropping one of them would break the master silently.
var locustStatsEntryFields = []string{
	"name",
	"method",
	"last_request_timestamp",
	"start_time",
	"num_requests",
	"num_none_requests",
	"num_failures",
	"total_response_time",
	"max_response_time",
	"min_response_time",
	"total_content_length",
	"response_times",
	"num_reqs_per_sec",
	"num_fail_per_sec",
}

func TestStatsReportFieldsMatchLocust(t *testing.T) {
	stats := newRequestStats()
	stats.logRequest("http", "success", 10, 20)
	stats.logError("http", "failure", "500 error")

	data := stats.collectReportData()

	entries, ok := data["stats"].([]interface{})
	if !ok || len(entries) == 0 {
		t.Fatalf("expected a non empty stats list, got %v", data["stats"])
	}
	for _, field := range locustStatsEntryFields {
		if _, ok := entries[0].(map[string]interface{})[field]; !ok {
			t.Fatalf("stats entry is missing the field %q required by locust", field)
		}
	}

	total, ok := data["stats_total"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stats_total to be a map, got %T", data["stats_total"])
	}
	for _, field := range locustStatsEntryFields {
		if _, ok := total[field]; !ok {
			t.Fatalf("stats_total is missing the field %q required by locust", field)
		}
	}

	errors, ok := data["errors"].(map[string]map[string]interface{})
	if !ok || len(errors) != 1 {
		t.Fatalf("expected exactly one error entry, got %v", data["errors"])
	}
	for _, entry := range errors {
		for _, field := range []string{"method", "name", "error", "occurrences"} {
			if _, ok := entry[field]; !ok {
				t.Fatalf("error entry is missing the field %q required by locust", field)
			}
		}
	}
}
