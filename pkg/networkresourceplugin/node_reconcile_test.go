// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

var _ = Describe("desired network resources", Label("networkresourceplugin_node_reconcile_test"), func() {
	It("returns no desired resources when the plugin is disabled", func() {
		resources, err := ComputeDesiredResources(true, nil, []string{"eth0"}, Config{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(BeNil())
	})

	It("returns no desired resources for nodes that do not match device plugin affinity", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"role": "compute"}}}
		resources, err := ComputeDesiredResources(true, node, []string{"eth0"}, Config{
			Enabled: true,
			DevicePluginAffinity: DevicePluginAffinityConfig{
				NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "network"}},
			},
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI:    SubENIAdvertisementConfig{Rules: []SubENIRuleConfig{{ResourceName: constant.DefaultENISlotResourceName, DefaultMaxCount: 2}}},
				MasterNIC: MasterNICAdvertisementConfig{Rules: []MasterNICRuleConfig{{}}},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(BeNil())
	})

	It("computes sub-ENI resources only when provider mode is enabled and sub-ENI rules are configured", func() {
		cfg := Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 2,
					}},
				},
			},
		}

		resources, err := ComputeDesiredResources(false, nil, nil, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(BeEmpty())

		resources, err = ComputeDesiredResources(true, nil, nil, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      2,
		}}))
	})

	It("computes sub-ENI resources only for selected nodes", func() {
		cfg := Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 2,
						NodeSelector:    metav1.LabelSelector{MatchLabels: map[string]string{"role": "network"}},
					}},
				},
			},
		}

		resources, err := ComputeDesiredResources(true, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"role": "compute"},
		}}, nil, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(BeEmpty())

		resources, err = ComputeDesiredResources(true, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"role": "network"},
		}}, nil, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      2,
		}}))
	})

	It("computes sub-ENI resources with match expression selectors", func() {
		cfg := Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 2,
						NodeSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "role",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"network", "storage"},
							},
							{
								Key:      "maintenance",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"true"},
							},
						}},
					}},
				},
			},
		}

		resources, err := ComputeDesiredResources(true, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"role": "network", "maintenance": "false"},
		}}, nil, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      2,
		}}))
	})

	It("computes master NIC resources independently of provider mode", func() {
		resources, err := ComputeDesiredResources(false, nil, []string{"eth0", "ens1"}, Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				MasterNIC: MasterNICAdvertisementConfig{
					Rules: []MasterNICRuleConfig{{
						IncludeInterfaces: []string{"eth*"},
					}},
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: "spidernet.io/eth0-nic",
			Devices:      DefaultMasterNICMaxCount,
			Interface:    "eth0",
		}}))
	})

	It("uses defaultMaxCount for master NIC capacity", func() {
		resources, err := ComputeDesiredResources(false, nil, []string{"eth0", "ens1"}, Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				MasterNIC: MasterNICAdvertisementConfig{
					Rules: []MasterNICRuleConfig{{
						DefaultMaxCount:   32,
						IncludeInterfaces: []string{"eth*"},
					}},
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: "spidernet.io/eth0-nic",
			Devices:      32,
			Interface:    "eth0",
		}}))
	})

	It("combines sub-ENI and master NIC resources in deterministic order", func() {
		resources, err := ComputeDesiredResources(true, nil, []string{"ens1", "eth0"}, Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 1,
					}},
				},
				MasterNIC: MasterNICAdvertisementConfig{Rules: []MasterNICRuleConfig{{}}},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{
			{ResourceName: constant.DefaultENISlotResourceName, Devices: 1},
			{ResourceName: "spidernet.io/ens1-nic", Devices: DefaultMasterNICMaxCount, Interface: "ens1"},
			{ResourceName: "spidernet.io/eth0-nic", Devices: DefaultMasterNICMaxCount, Interface: "eth0"},
		}))
	})

	It("uses defaultMaxCount for sub-ENI capacity", func() {
		resources, err := ComputeDesiredResources(true, &corev1.Node{}, nil, Config{
			Enabled: true,
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{
						ResourceName:    constant.DefaultENISlotResourceName,
						DefaultMaxCount: 1,
					}},
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resources).To(Equal([]DesiredResource{{
			ResourceName: constant.DefaultENISlotResourceName,
			Devices:      1,
		}}))
	})
})
