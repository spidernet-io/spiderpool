// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package configmap_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
)

func TestConfigMap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configmap Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT())
	Expect(e).NotTo(HaveOccurred())
})
