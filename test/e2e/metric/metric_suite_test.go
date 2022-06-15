// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package metric_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

func TestMetric(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metric Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT(),[]
	{
		spiderpool.AddToScheme
	} )
	Expect(e).NotTo(HaveOccurred())
})
