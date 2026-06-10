// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

var _ = Describe("Controller config ENI device plugin", Label("controller_eni_config_test"), func() {
	loadConfig := func(contents string) error {
		file, err := os.CreateTemp(GinkgoT().TempDir(), "conf-*.yml")
		Expect(err).NotTo(HaveOccurred())
		_, err = file.WriteString(contents)
		Expect(err).NotTo(HaveOccurred())
		Expect(file.Close()).To(Succeed())

		cc := &ControllerContext{}
		cc.Cfg.ConfigPath = file.Name()
		err = cc.LoadConfigmap()
		controllerContext.Cfg = cc.Cfg
		return err
	}

	It("parses the default disabled state", func() {
		err := loadConfig("iaasNetworkProvider:\n  serverUrl: \"\"\n")

		Expect(err).NotTo(HaveOccurred())
		Expect(controllerContext.Cfg.IaaSProviderConfig.ENIDevPlugin.Enabled).To(BeFalse())
		Expect(controllerContext.Cfg.IaaSProviderConfig.ENIDevPlugin.ResourceName).To(Equal(constant.DefaultENISlotResourceName))
		Expect(controllerContext.Cfg.IaaSProviderConfig.ENIDevPlugin.KubeletRootDir).To(Equal("/var/lib/kubelet"))
		Expect(controllerContext.Cfg.IaaSProviderConfig.ENIDevPlugin.InjectPodENIResources).NotTo(BeNil())
		Expect(*controllerContext.Cfg.IaaSProviderConfig.ENIDevPlugin.InjectPodENIResources).To(BeTrue())
	})

	It("rejects an invalid resource name", func() {
		err := loadConfig("iaasNetworkProvider:\n  serverUrl: http://provider.example\n  eniDevPlugin:\n    enabled: true\n    resourceName: bad*resource\n")

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("resourceName"))
	})

	It("rejects negative max slots", func() {
		err := loadConfig("iaasNetworkProvider:\n  serverUrl: http://provider.example\n  eniDevPlugin:\n    enabled: true\n    maxSlotsPerNode: -1\n")

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("maxSlotsPerNode"))
	})

	It("allows the feature to stay inactive without provider configuration", func() {
		err := loadConfig("iaasNetworkProvider:\n  eniDevPlugin:\n    enabled: true\n")

		Expect(err).NotTo(HaveOccurred())
		Expect(controllerContext.Cfg.IaaSProviderConfig.ENIDevPlugin.Enabled).To(BeTrue())
	})

	It("rejects a relative kubelet root", func() {
		err := loadConfig("iaasNetworkProvider:\n  serverUrl: http://provider.example\n  eniDevPlugin:\n    enabled: true\n    kubeletRootDir: var/lib/kubelet\n")

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kubeletRootDir"))
	})
})
