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
		newStats.logRequest("http", "success", 1, 20)
		newStats.clearStatsChan <- true
		Expect(newStats.total.NumRequests).To(BeEquivalentTo(0))
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

	It("test collect report data", func() {
		newStats := newRequestStats()
		newStats.logRequest("http", "success", 2, 30)
		newStats.logError("http", "failure", "500 error")
		result := newStats.collectReportData()

		Expect(result).To(HaveKey("stats"))
		Expect(result).To(HaveKey("stats_total"))
		Expect(result).To(HaveKey("errors"))
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
