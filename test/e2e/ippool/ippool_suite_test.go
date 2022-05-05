// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework"
)

func TestIppool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ippool Suite")
}

var frame *e2eframework.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2eframework.NewFramework(GinkgoT())
	Expect(e).NotTo(HaveOccurred())
})
