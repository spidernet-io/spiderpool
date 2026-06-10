// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package eni_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestENIDevicePlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ENI Device Plugin Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var err error
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme})
	Expect(err).NotTo(HaveOccurred())
})
