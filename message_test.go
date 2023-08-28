package boomer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test message", func() {

	It("test encode and decode", func() {
		data := make(map[string]interface{})
		data["a"] = 1
		data["b"] = "hello"
		msg := newGenericMessage("test", data, "nodeID")

		encoded, err := msg.serialize()
		Expect(err).NotTo(HaveOccurred())

		decoded, err := newGenericMessageFromBytes(encoded)
		Expect(err).NotTo(HaveOccurred())

		Expect(msg.Type).To(Equal(decoded.Type))
		Expect(msg.NodeID).To(Equal(decoded.NodeID))

		decodedA := decoded.Data["a"]
		decodedAInt := decodedA.(int64)
		decodedB := decoded.Data["b"]
		decodedBArray := decodedB.([]uint8)

		Expect(msg.Data["a"]).To(BeEquivalentTo(decodedAInt))
		Expect(msg.Data["b"]).To(BeEquivalentTo(decodedBArray))
	})
})
