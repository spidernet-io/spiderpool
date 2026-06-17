// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"k8s.io/apimachinery/pkg/runtime"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

func TestNetworkResourcePlugin(t *testing.T) {
	if !common.IsIaaSNetworkProviderEnabled() {
		t.Skip("network resource plugin e2e requires provider mode; set E2E_IAAS_NETWORK_PROVIDER_ENABLED=true to run this suite")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Resource Plugin E2E Suite")
}

var frame *e2e.Framework

var _ = BeforeSuite(func() {
	defer GinkgoRecover()

	var err error
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme})
	Expect(err).NotTo(HaveOccurred())
})
