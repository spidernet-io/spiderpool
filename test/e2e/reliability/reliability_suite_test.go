// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reliability_test

import (
	"testing"

	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
)

func TestReliability(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reliability Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme, coordinationv1.AddToScheme})
	Expect(e).NotTo(HaveOccurred())

})
