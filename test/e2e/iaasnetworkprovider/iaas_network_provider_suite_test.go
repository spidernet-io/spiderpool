// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package iaasnetworkprovider_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIaaSNetworkProvider(t *testing.T) {
	if !common.IsIaaSNetworkProviderEnabled() {
		t.Skip("IaaS network provider e2e is disabled; set E2E_IAAS_NETWORK_PROVIDER_ENABLED=true to run this suite")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "IaaS Network Provider Suite")
}

var (
	frame             *e2e.Framework
	providerMock      *providerMockServer
	providerMockURL   string
	providerNamespace string
)

var _ = BeforeSuite(func() {
	defer GinkgoRecover()

	var err error
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme})
	Expect(err).NotTo(HaveOccurred())

	providerNamespace = providerMockNamespaceForProcess()
	providerMock = newProviderMockServer(frame, providerNamespace)
	providerMockURL, err = providerMock.Deploy()
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Printf("IaaS provider mock server is available at %s\n", providerMockURL)
})

var _ = BeforeEach(func() {
	if !frame.Info.IpV4Enabled || frame.Info.IpV6Enabled {
		Skip("IaaS network provider e2e requires an IPv4-only cluster because the current provider-mode cases use IPv4 IPPools")
	}
})

var _ = AfterSuite(func() {
	if providerMock == nil {
		return
	}
	Expect(providerMock.Cleanup()).To(Succeed())
})
