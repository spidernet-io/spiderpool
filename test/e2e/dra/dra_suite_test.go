// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package dra_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/api/resource/v1alpha2"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDra(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dra Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	if !common.IsDRAEnabled() {
		GinkgoWriter.Println("DRA feature is disabled. Skip")
		Skip("DRA feature is disabled. Skip")
	}

	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme, v1alpha2.AddToScheme})
	Expect(e).NotTo(HaveOccurred())
})
