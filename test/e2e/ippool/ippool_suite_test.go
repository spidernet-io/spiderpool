package ippool_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spidernet-io/spiderpool/test/e2e/framework"
)

func TestIppool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ippool Suite")
}

var frame *Framework

var _ = BeforeSuite(func() {
	var e error
	frame = NewFramework()
	if e != nil {
		Fail("failed to initialize framework")
	}
})
