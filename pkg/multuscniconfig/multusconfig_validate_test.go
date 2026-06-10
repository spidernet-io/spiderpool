// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var _ = Describe("MultusConfigValidate", Label("multusconfig_validate_test", "unittest"), func() {
	DescribeTable(
		"network-resource-inject validation",
		func(multusConfig *spiderpoolv2beta1.SpiderMultusConfig) {
			Expect(validateCNIConfig(multusConfig)).To(Succeed())
		},
		Entry("macvlan allows network-resource-inject without rdmaResourceName", &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "macvlan-test",
				Namespace: "default",
				Annotations: map[string]string{
					constant.AnnoNetworkResourceInject: "true",
				},
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					Master: []string{"eth0"},
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"pool-a"},
					},
				},
			},
		}),
		Entry("ipvlan allows network-resource-inject without rdmaResourceName", &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ipvlan-test",
				Namespace: "default",
				Annotations: map[string]string{
					constant.AnnoNetworkResourceInject: "true",
				},
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.IPVlanCNI),
				IPVlanConfig: &spiderpoolv2beta1.SpiderIPvlanCniConfig{
					Master: []string{"eth0"},
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"pool-a"},
					},
				},
			},
		}),
		Entry("sriov allows network-resource-inject with resourceName and pools", &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sriov-test",
				Namespace: "default",
				Annotations: map[string]string{
					constant.AnnoNetworkResourceInject: "true",
				},
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.SriovCNI),
				SriovConfig: &spiderpoolv2beta1.SpiderSRIOVCniConfig{
					ResourceName: ptr.To("vendor.io/resource"),
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"pool-a"},
					},
				},
			},
		}),
	)
})
