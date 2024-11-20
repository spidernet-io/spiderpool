// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package subnet_test

import (
	"testing"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSubnet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Subnet Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme})

	Expect(e).NotTo(HaveOccurred())

	if !common.CheckSubnetFeatureOn() {
		Skip("Subnet feature is off , ignore this suite")
	}
})
