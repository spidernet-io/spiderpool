// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"k8s.io/utils/ptr"
)

var _ = Describe("SpiderMultusConfig spec.ipam", Label("spidermultusconfig", "unittest"), func() {
	newMacvlanSpec := func(disableIPAM *bool, ipam *spiderpoolv2beta1.SpiderIPAMConfig) *spiderpoolv2beta1.MultusCNIConfigSpec {
		return &spiderpoolv2beta1.MultusCNIConfigSpec{
			CniType:     ptr.To(constant.MacvlanCNI),
			DisableIPAM: disableIPAM,
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
