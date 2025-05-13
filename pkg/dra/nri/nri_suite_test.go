package nri

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNri(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NRI Suite")
}
