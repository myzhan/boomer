package boomer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test legacy", func() {

	It("test convert response time", func() {
		convertedFloat := convertResponseTime(float64(1.234))
		Expect(convertedFloat).To(BeEquivalentTo(1))
		convertedInt64 := convertResponseTime(int64(2))
		Expect(convertedInt64).To(BeEquivalentTo(2))

		Expect(func() {
			convertResponseTime(1)
		}).Should(Panic())
	})

	It("test init events", func() {
		initLegacyEventHandlers()
		defer Events.Unsubscribe("request_success", legacySuccessHandler)
		defer Events.Unsubscribe("request_failure", legacyFailureHandler)

		masterHost := "127.0.0.1"
		masterPort := 5557
		defaultBoomer = NewBoomer(masterHost, masterPort)
		defaultBoomer.slaveRunner = newSlaveRunner(masterHost, masterPort, nil, nil)

		Events.Publish("request_success", "http", "foo", int64(1), int64(10))
		Events.Publish("request_failure", "udp", "bar", int64(2), "udp error")

		requestSuccessMsg := <-defaultBoomer.slaveRunner.stats.requestSuccessChan
		Expect(requestSuccessMsg.requestType).To(Equal("http"))
		Expect(requestSuccessMsg.responseTime).To(BeEquivalentTo(1))

		requestFailureMsg := <-defaultBoomer.slaveRunner.stats.requestFailureChan
		Expect(requestFailureMsg.requestType).To(Equal("udp"))
		Expect(requestFailureMsg.responseTime).To(BeEquivalentTo(2))
		Expect(requestFailureMsg.error).To(Equal("udp error"))
	})
})
