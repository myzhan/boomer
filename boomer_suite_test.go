package boomer_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBoomer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Boomer Suite")
}
