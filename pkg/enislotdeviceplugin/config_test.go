// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("ENI device plugin config", Label("enislotdeviceplugin_config_test"), func() {
	It("returns safe defaults when cfg is nil", func() {
		result, err := ApplyDefaultsAndValidate(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Enabled).To(BeFalse())
		Expect(result.ResourceName).To(Equal(constant.DefaultENISlotResourceName))
		Expect(result.KubeletRootDir).To(Equal(DefaultKubeletRootDir))
		Expect(result.InjectPodENIResources).To(BeTrue())
	})

	It("cleans a kubelet root dir with a trailing slash", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				KubeletRootDir: "/var/lib/kubelet/",
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.KubeletRootDir).To(Equal("/var/lib/kubelet"))
	})

	It("defaults to disabled with the canonical resource name and enabled webhook injection", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Enabled).To(BeFalse())
		Expect(result.ResourceName).To(Equal(constant.DefaultENISlotResourceName))
		Expect(result.KubeletRootDir).To(Equal(DefaultKubeletRootDir))
		Expect(result.InjectPodENIResources).To(BeTrue())
		Expect(cfg.ENIDevPlugin.ResourceName).To(Equal(constant.DefaultENISlotResourceName))
		Expect(cfg.ENIDevPlugin.KubeletRootDir).To(Equal(DefaultKubeletRootDir))
		Expect(cfg.ENIDevPlugin.InjectPodENIResources).NotTo(BeNil())
		Expect(*cfg.ENIDevPlugin.InjectPodENIResources).To(BeTrue())
	})

	It("keeps an explicit false webhook injection setting", func() {
		inject := false
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				InjectPodENIResources: &inject,
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.InjectPodENIResources).To(BeFalse())
	})

	It("is active only when provider mode and the feature are enabled", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ServerURL: "http://provider.example",
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled: true,
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Enabled).To(BeTrue())
	})

	It("rejects an invalid resource name when enabled", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ServerURL: "http://provider.example",
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled:      true,
				ResourceName: "bad*resource",
			},
		}

		_, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("resourceName"))
	})

	It("rejects negative max slots when enabled", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ServerURL: "http://provider.example",
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled:         true,
				MaxSlotsPerNode: -1,
			},
		}

		_, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("maxSlotsPerNode"))
	})

	It("stays inactive when the feature is enabled without provider mode", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled: true,
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Enabled).To(BeTrue())
	})

	It("rejects a relative kubelet root when enabled", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ServerURL: "http://provider.example",
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled:        true,
				KubeletRootDir: "var/lib/kubelet",
			},
		}

		_, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kubeletRootDir"))
	})

	It("keeps an explicit absolute kubelet root", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ServerURL: "http://provider.example",
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled:        true,
				KubeletRootDir: "/var/log/kubelet",
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.KubeletRootDir).To(Equal("/var/log/kubelet"))
	})

	It("keeps all explicit valid device plugin settings", func() {
		inject := false
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				Enabled:               true,
				ResourceName:          "example.com/custom-eni",
				MaxSlotsPerNode:       7,
				KubeletRootDir:        "/custom/kubelet",
				InjectPodENIResources: &inject,
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(&Config{
			Enabled:               true,
			ResourceName:          "example.com/custom-eni",
			MaxSlotsPerNode:       7,
			KubeletRootDir:        "/custom/kubelet",
			InjectPodENIResources: false,
		}))
	})

	It("cleans dot segments from kubelet root dir before validating", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				KubeletRootDir: "/var/lib/kubelet/../kubelet",
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.KubeletRootDir).To(Equal("/var/lib/kubelet"))
	})

	It("accepts zero max slots as a valid explicit limit", func() {
		cfg := &spiderpooltypes.IaaSProviderConfig{
			ENIDevPlugin: spiderpooltypes.ENIDevPluginConfig{
				MaxSlotsPerNode: 0,
			},
		}

		result, err := ApplyDefaultsAndValidate(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.MaxSlotsPerNode).To(Equal(0))
	})
})
