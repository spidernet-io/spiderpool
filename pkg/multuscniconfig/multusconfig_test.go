// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	"k8s.io/utils/ptr"
)

func TestMultusConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MultusConfig Suite")
}

var _ = Describe("SpiderMultusConfig vlan", Label("spidermultusconfig", "unittest"), func() {
	newVlanSMC := func(vlanID *int32) *spiderpoolv2beta1.SpiderMultusConfig {
		return &spiderpoolv2beta1.SpiderMultusConfig{
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.VlanCNI),
				VlanConfig: &spiderpoolv2beta1.SpiderVlanCniConfig{
					Master: []string{"eth0"},
					VlanID: vlanID,
				},
				ChainCNIJsonData: []string{},
			},
		}
	}

	It("requires a static vlanID", func() {
		smc := newVlanSMC(nil)
		mutateSpiderMultusConfig(logutils.IntoContext(context.Background(), zap.NewNop()), smc)

		err := validateCNIConfig(smc)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("vlanID is required"))
	})

	It("renders the static vlanId in the generated vlan CNI config", func() {
		smc := newVlanSMC(ptr.To(int32(100)))
		mutateSpiderMultusConfig(logutils.IntoContext(context.Background(), zap.NewNop()), smc)
		Expect(validateCNIConfig(smc)).To(BeNil())

		conf := generateVlanCNIConf(false, smc.Spec)
		data, err := json.Marshal(conf)
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(data, &decoded)).To(Succeed())
		Expect(decoded).To(HaveKeyWithValue("type", constant.VlanCNI))
		Expect(decoded).To(HaveKeyWithValue("master", "eth0"))
		Expect(decoded).To(HaveKeyWithValue("vlanId", float64(100)))
		Expect(decoded).NotTo(HaveKey("vlanMode"))
	})
})

var _ = Describe("SpiderMultusConfig eni-vlan", Label("spidermultusconfig", "unittest"), func() {
	newEniVlanSMC := func() *spiderpoolv2beta1.SpiderMultusConfig {
		return &spiderpoolv2beta1.SpiderMultusConfig{
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.EniVlanCNI),
				EniVlanConfig: &spiderpoolv2beta1.SpiderEniVlanCniConfig{
					Master: []string{"eth1"},
				},
				ChainCNIJsonData: []string{},
			},
		}
	}

	It("mutates the connectivity validation defaults", func() {
		smc := newEniVlanSMC()
		mutateSpiderMultusConfig(logutils.IntoContext(context.Background(), zap.NewNop()), smc)

		Expect(smc.Spec.EniVlanConfig.ValidateIaasNetConfig).To(HaveValue(BeFalse()))
		Expect(smc.Spec.EniVlanConfig.ValidationRetries).To(HaveValue(Equal(int32(3))))
		Expect(smc.Spec.EniVlanConfig.ValidationTimeoutMs).To(HaveValue(Equal(int32(500))))
		Expect(validateCNIConfig(smc)).To(BeNil())
	})

	It("requires master", func() {
		smc := newEniVlanSMC()
		smc.Spec.EniVlanConfig.Master = nil

		err := validateCNIConfig(smc)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("master can't be empty"))
	})

	It("forbids disabling the spiderpool IPAM plugin", func() {
		smc := newEniVlanSMC()
		smc.Spec.IPAM = &spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(false)}

		err := validateCNIConfig(smc)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("strongly depends on the spiderpool IPAM plugin"))

		smc = newEniVlanSMC()
		smc.Spec.DisableIPAM = ptr.To(true) //nolint:staticcheck // SA1019: verify the deprecated spec.disableIPAM is rejected too.
		err = validateCNIConfig(smc)
		Expect(err).NotTo(BeNil())
	})

	It("forbids coexistence with other CNI config sections", func() {
		smc := newEniVlanSMC()
		smc.Spec.VlanConfig = &spiderpoolv2beta1.SpiderVlanCniConfig{
			Master: []string{"eth0"},
			VlanID: ptr.To(int32(100)),
		}

		err := validateCNIConfig(smc)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("please remove other CNI configs"))
	})

	It("renders the eni-vlan CNI config with forced-off IPAM detections", func() {
		smc := newEniVlanSMC()
		smc.Spec.EniVlanConfig.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{"pool-eth1"},
		}
		mutateSpiderMultusConfig(logutils.IntoContext(context.Background(), zap.NewNop()), smc)

		conf := generateEniVlanCNIConf(smc.Spec)
		data, err := json.Marshal(conf)
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(data, &decoded)).To(Succeed())
		Expect(decoded).To(HaveKeyWithValue("type", constant.EniVlanCNI))
		Expect(decoded).To(HaveKeyWithValue("master", "eth1"))
		Expect(decoded).To(HaveKeyWithValue("validateIaasNetConfig", false))
		Expect(decoded).To(HaveKeyWithValue("validationRetries", float64(3)))
		Expect(decoded).To(HaveKeyWithValue("validationTimeoutMs", float64(500)))
		Expect(decoded).NotTo(HaveKey("vlanId"))

		ipam, ok := decoded["ipam"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(ipam).To(HaveKeyWithValue("type", constant.Spiderpool))
		Expect(ipam).To(HaveKeyWithValue("default_ipv4_ippool", ConsistOf("pool-eth1")))
	})

	It("applies the connectivity validation defaults when the SMC doesn't carry them", func() {
		smc := newEniVlanSMC()

		conf := generateEniVlanCNIConf(smc.Spec)
		data, err := json.Marshal(conf)
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(data, &decoded)).To(Succeed())
		Expect(decoded).To(HaveKeyWithValue("validateIaasNetConfig", false))
		Expect(decoded).To(HaveKeyWithValue("validationRetries", float64(3)))
		Expect(decoded).To(HaveKeyWithValue("validationTimeoutMs", float64(500)))
	})
})

var _ = Describe("SpiderMultusConfig spec.ipam", Label("spidermultusconfig", "unittest"), func() {
	newMacvlanSpec := func(disableIPAM *bool, ipam *spiderpoolv2beta1.SpiderIPAMConfig) *spiderpoolv2beta1.MultusCNIConfigSpec {
		return &spiderpoolv2beta1.MultusCNIConfigSpec{
			CniType:     ptr.To(constant.MacvlanCNI),
			DisableIPAM: disableIPAM, //nolint:staticcheck // SA1019: verify compatibility with deprecated spec.disableIPAM.
			IPAM:        ipam,
			MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
				Master: []string{"eth0"},
			},
		}
	}

	ipamEnabled := func(spec *spiderpoolv2beta1.MultusCNIConfigSpec) bool {
		return !isIPAMDisabled(spec)
	}

	DescribeTable("spec.ipam.enabled and the deprecated spec.disableIPAM precedence",
		func(disableIPAM *bool, ipam *spiderpoolv2beta1.SpiderIPAMConfig, expectEnabled bool) {
			Expect(ipamEnabled(newMacvlanSpec(disableIPAM, ipam))).To(Equal(expectEnabled))
		},
		Entry("both unset defaults to enabled", nil, nil, true),
		Entry("disableIPAM=false keeps IPAM enabled", ptr.To(false), nil, true),
		Entry("disableIPAM=true disables IPAM", ptr.To(true), nil, false),
		Entry("ipam block without enabled falls back to disableIPAM=true", ptr.To(true),
			&spiderpoolv2beta1.SpiderIPAMConfig{}, false),
		Entry("ipam.enabled=true overrides disableIPAM=true", ptr.To(true),
			&spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(true)}, true),
		Entry("ipam.enabled=false overrides disableIPAM=false", ptr.To(false),
			&spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(false)}, false),
		Entry("ipam.enabled=false with disableIPAM unset disables IPAM", nil,
			&spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(false)}, false),
		Entry("ipam.enabled=true with disableIPAM unset keeps IPAM enabled", nil,
			&spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(true)}, true),
	)

	// getGeneratedPlugins renders the SpiderMultusConfig into its
	// NetworkAttachmentDefinition and returns the decoded CNI plugin list.
	getGeneratedPlugins := func(spec *spiderpoolv2beta1.MultusCNIConfigSpec) []map[string]interface{} {
		GinkgoHelper()
		spec.EnableCoordinator = ptr.To(false)
		nad, err := generateNetAttachDef("test-nad", &spiderpoolv2beta1.SpiderMultusConfig{
			Spec: *spec,
		})
		Expect(err).NotTo(HaveOccurred())

		var rawList struct {
			Plugins []map[string]interface{} `json:"plugins"`
		}
		Expect(json.Unmarshal([]byte(nad.Spec.Config), &rawList)).To(Succeed())
		return rawList.Plugins
	}

	It("generates the NAD with the ipam section when spec.ipam.enabled=true", func() {
		plugins := getGeneratedPlugins(newMacvlanSpec(ptr.To(true), &spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(true)}))
		Expect(plugins).NotTo(BeEmpty())
		Expect(plugins[0]).To(HaveKeyWithValue("type", constant.MacvlanCNI))
		Expect(plugins[0]).To(HaveKey("ipam"))
		ipam := plugins[0]["ipam"].(map[string]interface{})
		Expect(ipam).To(HaveKeyWithValue("type", constant.Spiderpool))
	})

	It("generates the NAD without the ipam section when spec.ipam.enabled=false", func() {
		plugins := getGeneratedPlugins(newMacvlanSpec(nil, &spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(false)}))
		Expect(plugins).NotTo(BeEmpty())
		Expect(plugins[0]).To(HaveKeyWithValue("type", constant.MacvlanCNI))
		Expect(plugins[0]).NotTo(HaveKey("ipam"))
	})

	It("generates the NAD without the ipam section when the deprecated disableIPAM=true", func() {
		plugins := getGeneratedPlugins(newMacvlanSpec(ptr.To(true), nil))
		Expect(plugins).NotTo(BeEmpty())
		Expect(plugins[0]).NotTo(HaveKey("ipam"))
	})

	It("spec.ipam.enabled takes precedence over the deprecated spec.disableIPAM", func() {
		Expect(ipamEnabled(newMacvlanSpec(nil, nil))).To(BeTrue())
		Expect(ipamEnabled(newMacvlanSpec(ptr.To(true), nil))).To(BeFalse())
		Expect(ipamEnabled(newMacvlanSpec(ptr.To(true), &spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(true)}))).To(BeTrue())
		Expect(ipamEnabled(newMacvlanSpec(ptr.To(false), &spiderpoolv2beta1.SpiderIPAMConfig{Enabled: ptr.To(false)}))).To(BeFalse())
	})

	It("translates spec.ipam.logOptions into the ipam CNI configuration", func() {
		spec := newMacvlanSpec(nil, &spiderpoolv2beta1.SpiderIPAMConfig{
			Enabled: ptr.To(true),
			LogOptions: &spiderpoolv2beta1.LogOptions{
				LogLevel:        ptr.To("info"),
				LogFilePath:     ptr.To("/var/log/spidernet/custom.log"),
				LogFileMaxSize:  ptr.To(int32(50)),
				LogFileMaxAge:   ptr.To(int32(7)),
				LogFileMaxCount: ptr.To(int32(3)),
			},
		})

		raw, err := json.Marshal(generateMacvlanCNIConf(false, *spec))
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		ipam, ok := decoded["ipam"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(ipam).To(HaveKeyWithValue("type", constant.Spiderpool))
		Expect(ipam).To(HaveKeyWithValue("log_level", "info"))
		Expect(ipam).To(HaveKeyWithValue("log_file_path", "/var/log/spidernet/custom.log"))
		Expect(ipam).To(HaveKeyWithValue("log_file_max_size", float64(50)))
		Expect(ipam).To(HaveKeyWithValue("log_file_max_age", float64(7)))
		Expect(ipam).To(HaveKeyWithValue("log_file_max_count", float64(3)))
	})

	It("omits unset log options in the ipam CNI configuration", func() {
		spec := newMacvlanSpec(nil, nil)
		raw, err := json.Marshal(generateMacvlanCNIConf(false, *spec))
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		ipam, ok := decoded["ipam"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(ipam).NotTo(HaveKey("log_level"))
		Expect(ipam).NotTo(HaveKey("log_file_max_size"))
		Expect(ipam).NotTo(HaveKey("log_file_max_age"))
		Expect(ipam).NotTo(HaveKey("log_file_max_count"))
	})

	It("translates spec.coordinator.logOptions into the coordinator CNI configuration", func() {
		coordinatorSpec := &spiderpoolv2beta1.CoordinatorSpec{
			LogOptions: &spiderpoolv2beta1.LogOptions{
				LogLevel:        ptr.To("error"),
				LogFilePath:     ptr.To("/var/log/spidernet/coordinator-custom.log"),
				LogFileMaxSize:  ptr.To(int32(20)),
				LogFileMaxAge:   ptr.To(int32(5)),
				LogFileMaxCount: ptr.To(int32(2)),
			},
		}

		raw, err := json.Marshal(generateCoordinatorCNIConf(coordinatorSpec))
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		logOptions, ok := decoded["logOptions"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(logOptions).To(HaveKeyWithValue("logLevel", "error"))
		Expect(logOptions).To(HaveKeyWithValue("logFile", "/var/log/spidernet/coordinator-custom.log"))
		Expect(logOptions).To(HaveKeyWithValue("logMaxSize", float64(20)))
		Expect(logOptions).To(HaveKeyWithValue("logMaxAge", float64(5)))
		Expect(logOptions).To(HaveKeyWithValue("logMaxCount", float64(2)))
	})

	It("omits logOptions in the coordinator CNI configuration when unset", func() {
		raw, err := json.Marshal(generateCoordinatorCNIConf(nil))
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		Expect(decoded).NotTo(HaveKey("logOptions"))
	})
})
