package boomer

import (
	"os"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test utils", func() {

	DescribeTable("cast to int64", func(value interface{}, expect int64, ok bool) {
		result, o := castToInt64(value)
		Expect(o).To(Equal(ok))
		Expect(result).To(BeEquivalentTo(expect))
	},
		Entry("int64", int64(10), int64(10), true),
		Entry("uint64", uint64(10), int64(10), true),
		Entry("int32", int32(10), int64(0), false),
	)

	DescribeTable("test round", func(value float64, roundOn float64, places int, expect float64) {
		result := round(value, roundOn, places)
		Expect(result).To(Equal(expect))
	},
		Entry("147.5002", float64(147.5002), .5, -1, float64(150)),
		Entry("3432.5002", float64(3432.5002), .5, -2, float64(3400)),
	)

	It("test md5", func() {
		hashValue := MD5("Hello", "World!")
		Expect(hashValue).To(Equal("06e0e6637d27b2622ab52022db713ce2"))
	})

	It("test get nodeID", func() {
		nodeID := getNodeID()
		hostname, _ := os.Hostname()
		regex := hostname + "_[a-f0-9]{32}$"
		validNodeID := regexp.MustCompile(regex)
		Expect(validNodeID.MatchString(nodeID)).To(BeTrue())
	})

	It("test now", func() {
		now := Now()
		Expect(now >= 1000000000000 && now <= 2000000000000).To(BeTrue())
	})

	It("test start memory profile", func() {
		defer func() {
			os.Remove("mem.pprof")
		}()

		err := StartMemoryProfile("mem.pprof", 2*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Eventually("mem.pprof").WithTimeout(3 * time.Second).Should(BeAnExistingFile())
	})

	It("test start CPU profile", func() {
		defer func() {
			os.Remove("cpu.pprof")
		}()

		err := StartCPUProfile("cpu.pprof", 2*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Eventually("cpu.pprof").WithTimeout(3 * time.Second).Should(BeAnExistingFile())
	})
})
