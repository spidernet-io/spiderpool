// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("network resource plugin config", Label("networkresourceplugin_config_test"), func() {
	It("returns disabled defaults with canonical names", func() {
		cfg, err := ApplyDefaultsAndValidate(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Enabled).To(BeFalse())
		Expect(cfg.KubeletRootDir).To(Equal(DefaultKubeletRootDir))
		Expect(cfg.ResourceAdvertisement.SubENI.Rules).To(BeEmpty())
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules).To(BeEmpty())
	})

	It("applies configmap values and defensively copies master NIC rules", func() {
		input := &spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					Enabled:        true,
					KubeletRootDir: "/var/lib/custom-kubelet/../custom-kubelet",
					DevicePluginAffinity: spiderpooltypes.DevicePluginAffinity{
						NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"resource": "enabled"}},
					},
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						SubENI: spiderpooltypes.SubENIAdvertisement{
							Rules: []spiderpooltypes.SubENIRule{{
								ResourceName:    "example.com/sub-eni",
								DefaultMaxCount: 3,
								NodeSelector:    metav1.LabelSelector{MatchLabels: map[string]string{"role": "network"}},
							}},
						},
						MasterNIC: spiderpooltypes.MasterNICAdvertisement{
							Rules: []spiderpooltypes.MasterNICRule{{
								NodeSelector:      metav1.LabelSelector{MatchLabels: map[string]string{"zone": "east"}},
								DefaultMaxCount:   32,
								IncludeInterfaces: []string{"eth*", "ens*"},
								ExcludeInterfaces: []string{"eth9"},
							}},
						},
					},
				},
			},
		}

		cfg, err := ApplyDefaultsAndValidate(input)

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Enabled).To(BeTrue())
		Expect(cfg.KubeletRootDir).To(Equal("/var/lib/custom-kubelet"))
		Expect(cfg.DevicePluginAffinity.NodeSelector).To(Equal(metav1.LabelSelector{MatchLabels: map[string]string{"resource": "enabled"}}))
		Expect(cfg.ResourceAdvertisement.SubENI.Rules).To(Equal([]SubENIRuleConfig{{
			ResourceName:    "example.com/sub-eni",
			DefaultMaxCount: 3,
			NodeSelector:    metav1.LabelSelector{MatchLabels: map[string]string{"role": "network"}},
		}}))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules).To(HaveLen(1))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].NodeSelector).To(Equal(metav1.LabelSelector{MatchLabels: map[string]string{"zone": "east"}}))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].DefaultMaxCount).To(Equal(32))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].IncludeInterfaces).To(Equal([]string{"eth*", "ens*"}))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].ExcludeInterfaces).To(Equal([]string{"eth9"}))

		input.AgentConfig.NetworkResourcePlugin.ResourceAdvertisement.MasterNIC.Rules[0].NodeSelector.MatchLabels["zone"] = "west"
		input.AgentConfig.NetworkResourcePlugin.ResourceAdvertisement.MasterNIC.Rules[0].IncludeInterfaces[0] = "changed"
		input.AgentConfig.NetworkResourcePlugin.ResourceAdvertisement.SubENI.Rules[0].NodeSelector.MatchLabels["role"] = "changed"
		Expect(cfg.ResourceAdvertisement.SubENI.Rules[0].NodeSelector).To(Equal(metav1.LabelSelector{MatchLabels: map[string]string{"role": "network"}}))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].NodeSelector).To(Equal(metav1.LabelSelector{MatchLabels: map[string]string{"zone": "east"}}))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].IncludeInterfaces).To(Equal([]string{"eth*", "ens*"}))
	})

	It("defaults empty kubelet root, resource name, and master NIC capacity during validation", func() {
		cfg := &Config{
			ResourceAdvertisement: ResourceAdvertisementConfig{
				SubENI: SubENIAdvertisementConfig{
					Rules: []SubENIRuleConfig{{}},
				},
				MasterNIC: MasterNICAdvertisementConfig{
					Rules: []MasterNICRuleConfig{{}},
				},
			},
		}

		err := validate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.KubeletRootDir).To(Equal(DefaultKubeletRootDir))
		Expect(cfg.ResourceAdvertisement.SubENI.Rules[0].ResourceName).To(Equal(constant.DefaultENISlotResourceName))
		Expect(cfg.ResourceAdvertisement.MasterNIC.Rules[0].DefaultMaxCount).To(Equal(DefaultMasterNICMaxCount))
	})

	It("rejects invalid kubelet root dirs", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{KubeletRootDir: "relative"},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kubeletRootDir"))
	})

	It("rejects invalid extended resource names", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						SubENI: spiderpooltypes.SubENIAdvertisement{
							Rules: []spiderpooltypes.SubENIRule{{ResourceName: "bad*resource"}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("resourceName"))
	})

	It("rejects negative sub-ENI default capacity", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						SubENI: spiderpooltypes.SubENIAdvertisement{
							Rules: []spiderpooltypes.SubENIRule{{DefaultMaxCount: -1}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("defaultMaxCount"))
	})

	It("rejects invalid sub-ENI node selectors", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						SubENI: spiderpooltypes.SubENIAdvertisement{
							Rules: []spiderpooltypes.SubENIRule{{
								NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"bad/key/again": "value"}},
							}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nodeSelector"))

		_, err = ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						SubENI: spiderpooltypes.SubENIAdvertisement{
							Rules: []spiderpooltypes.SubENIRule{{
								NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "bad/value"}},
							}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nodeSelector"))
	})

	It("rejects invalid selectors and glob patterns", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					DevicePluginAffinity: spiderpooltypes.DevicePluginAffinity{
						NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"bad/key/again": "value"}},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("devicePluginAffinity.nodeSelector"))

		_, err = ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						MasterNIC: spiderpooltypes.MasterNICAdvertisement{
							Rules: []spiderpooltypes.MasterNICRule{{IncludeInterfaces: []string{"["}}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("pattern"))
	})

	It("rejects invalid master NIC rule node selectors", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						MasterNIC: spiderpooltypes.MasterNICAdvertisement{
							Rules: []spiderpooltypes.MasterNICRule{{
								NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{"bad/key/again": "value"}},
							}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nodeSelector"))
	})

	It("rejects negative master NIC default capacity", func() {
		_, err := ApplyDefaultsAndValidate(&spiderpooltypes.SpiderpoolConfigmapConfig{
			AgentConfig: spiderpooltypes.AgentConfig{
				NetworkResourcePlugin: spiderpooltypes.NetworkResourcePluginConfig{
					ResourceAdvertisement: spiderpooltypes.ResourceAdvertisement{
						MasterNIC: spiderpooltypes.MasterNICAdvertisement{
							Rules: []spiderpooltypes.MasterNICRule{{DefaultMaxCount: -1}},
						},
					},
				},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("defaultMaxCount"))
	})

	It("builds Kubernetes resource lists only for positive counts", func() {
		Expect(ResourceList("spidernet.io/sub-eni", 0)).To(BeNil())
		Expect(ResourceList("spidernet.io/sub-eni", -1)).To(BeNil())

		resources := ResourceList("spidernet.io/sub-eni", 2)
		Expect(resources).To(HaveLen(1))
		quantity := resources[corev1.ResourceName("spidernet.io/sub-eni")]
		Expect(quantity.String()).To(Equal("2"))
	})
})
