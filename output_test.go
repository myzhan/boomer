package boomer

import (
	"log"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test output", func() {

	It("test get median response time", func() {
		numRequests := int64(10)
		responseTimes := map[int64]int64{
			100: 1,
			200: 3,
			300: 6,
		}

		medianResponseTime := getMedianResponseTime(numRequests, responseTimes)
		Expect(medianResponseTime).To(BeEquivalentTo(300))

		responseTimes = map[int64]int64{}
		medianResponseTime = getMedianResponseTime(numRequests, responseTimes)
		Expect(medianResponseTime).To(BeEquivalentTo(0))
	})

	It("test get avg response time", func() {
		numRequests := int64(3)
		totalResponseTime := int64(100)

		avgResponseTime := getAvgResponseTime(numRequests, totalResponseTime)
		Expect(avgResponseTime).Should(BeNumerically("~", 33.333333333333336))

		avgResponseTime = getAvgResponseTime(int64(0), totalResponseTime)
		Expect(avgResponseTime).To(BeEquivalentTo(0))
	})

	It("test get avg content length", func() {
		numRequests := int64(3)
		totalContentLength := int64(100)

		avgContentLength := getAvgContentLength(numRequests, totalContentLength)
		Expect(avgContentLength).To(BeEquivalentTo(33))

		avgContentLength = getAvgContentLength(int64(0), totalContentLength)
		Expect(avgContentLength).To(BeEquivalentTo(0))
	})

	It("test get current rps", func() {
		numRequests := int64(10)
		numReqsPerSecond := map[int64]int64{}

		currentRps := getCurrentRps(numRequests, numReqsPerSecond)
		Expect(currentRps).To(BeEquivalentTo(0))

		numReqsPerSecond[1] = 2
		numReqsPerSecond[2] = 3
		numReqsPerSecond[3] = 2
		numReqsPerSecond[4] = 3

		currentRps = getCurrentRps(numRequests, numReqsPerSecond)
		Expect(currentRps).To(BeEquivalentTo(2))
	})

	It("test console output", func() {
		o := NewConsoleOutput()
		o.OnStart()

		data := map[string]interface{}{}
		stat := map[string]interface{}{}
		data["stats"] = []interface{}{stat}

		stat["name"] = "http"
		stat["method"] = "post"
		stat["num_requests"] = int64(100)
		stat["num_failures"] = int64(10)
		stat["response_times"] = map[int64]int64{
			10:  1,
			100: 99,
		}
		stat["total_response_time"] = int64(9910)
		stat["min_response_time"] = int64(10)
		stat["max_response_time"] = int64(100)
		stat["total_content_length"] = int64(100000)
		stat["num_reqs_per_sec"] = map[int64]int64{
			1: 20,
			2: 40,
			3: 40,
		}

		data["user_count"] = int32(10)
		data["stats_total"] = stat

		o.OnEvent(data)

		o.OnStop()
	})

	It("test loggers", func() {
		o := NewConsoleOutput()

		logger := log.New(os.Stdout, "[boomer]", log.LstdFlags)

		o.WithLogger(nil)
		o.WithLogger(logger)

		o2 := NewPrometheusPusherOutput("", "")
		o2.WithLogger(nil)
		o2.WithLogger(logger)
	})
})
