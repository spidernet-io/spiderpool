// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippoolmanager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IPPoolManagerConfig", Label("ippool_manager_test"), func() {
	var config IPPoolManagerConfig

	Describe("setDefaultsForIPPoolManagerConfig", func() {
		Context("when MaxAllocatedIPs is nil", func() {
			BeforeEach(func() {
				config = IPPoolManagerConfig{
					MaxAllocatedIPs:        nil,
					EnableKubevirtStaticIP: false,
				}
			})

			It("should set MaxAllocatedIPs to default value", func() {
				result := setDefaultsForIPPoolManagerConfig(config)
				Expect(result.MaxAllocatedIPs).NotTo(BeNil())
				Expect(*result.MaxAllocatedIPs).To(Equal(defaultMaxAllocatedIPs))
			})
		})

		Context("when MaxAllocatedIPs is set", func() {
			BeforeEach(func() {
				maxIPs := 3000
				config = IPPoolManagerConfig{
					MaxAllocatedIPs:        &maxIPs,
					EnableKubevirtStaticIP: true,
				}
			})

			It("should retain the provided MaxAllocatedIPs value", func() {
				result := setDefaultsForIPPoolManagerConfig(config)
				Expect(result.MaxAllocatedIPs).NotTo(BeNil())
				Expect(*result.MaxAllocatedIPs).To(Equal(3000))
			})
		})

		Context("when EnableKubevirtStaticIP is true", func() {
			BeforeEach(func() {
				config = IPPoolManagerConfig{
					MaxAllocatedIPs:        nil,
					EnableKubevirtStaticIP: true,
				}
			})

			It("should set MaxAllocatedIPs to default value", func() {
				result := setDefaultsForIPPoolManagerConfig(config)
				Expect(result.MaxAllocatedIPs).NotTo(BeNil())
				Expect(*result.MaxAllocatedIPs).To(Equal(defaultMaxAllocatedIPs))
				Expect(result.EnableKubevirtStaticIP).To(BeTrue())
			})
		})

		Context("when EnableKubevirtStaticIP is false", func() {
			BeforeEach(func() {
				config = IPPoolManagerConfig{
					MaxAllocatedIPs:        nil,
					EnableKubevirtStaticIP: false,
				}
			})

			It("should set MaxAllocatedIPs to default value", func() {
				result := setDefaultsForIPPoolManagerConfig(config)
				Expect(result.MaxAllocatedIPs).NotTo(BeNil())
				Expect(*result.MaxAllocatedIPs).To(Equal(defaultMaxAllocatedIPs))
				Expect(result.EnableKubevirtStaticIP).To(BeFalse())
			})
		})
	})
})
