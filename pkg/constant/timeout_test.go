// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTimeout(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Timeout Constant Suite")
}

var _ = Describe("Timeout Constants", Label("unitest"), func() {
	It("should have DefaultCNIClientTimeout equal to 100 seconds", Label("T013"), func() {
		Expect(DefaultCNIClientTimeout).To(Equal(100 * time.Second))
	})

	It("should have IaaSTimeoutStaticLimit equal to 2 minutes", func() {
		Expect(IaaSTimeoutStaticLimit).To(Equal(2 * time.Minute))
	})

	It("should have IaaSProviderRateLimitWait equal to 30 seconds", func() {
		Expect(IaaSProviderRateLimitWait).To(Equal(30 * time.Second))
	})

	It("should have IaaSProviderCloudAPITimeout equal to 16 seconds", func() {
		Expect(IaaSProviderCloudAPITimeout).To(Equal(16 * time.Second))
	})

	It("should have IaaSProviderWorstCase equal to rate-limit wait + cloud API + 2s margin", func() {
		Expect(IaaSProviderWorstCase).To(Equal(IaaSProviderRateLimitWait + IaaSProviderCloudAPITimeout + 2*time.Second))
	})

	It("should have DefaultIaaSProviderTimeout equal to 50 seconds", func() {
		Expect(DefaultIaaSProviderTimeout).To(Equal(50 * time.Second))
	})

	It("should ensure DefaultIaaSProviderTimeout is greater than IaaSProviderWorstCase", func() {
		Expect(DefaultIaaSProviderTimeout).To(BeNumerically(">", IaaSProviderWorstCase))
	})

	It("should ensure DefaultIaaSProviderTimeout is less than DefaultCNIClientTimeout", Label("SC-003"), func() {
		Expect(DefaultIaaSProviderTimeout).To(BeNumerically("<", DefaultCNIClientTimeout))
	})

	It("should ensure DefaultCNIClientTimeout is less than IaaSTimeoutStaticLimit", func() {
		Expect(DefaultCNIClientTimeout).To(BeNumerically("<", IaaSTimeoutStaticLimit))
	})
})
