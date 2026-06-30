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

var _ = Describe("SpiderMultusConfig vlan mode", Label("spidermultusconfig", "unittest"), func() {
	newVlanSMC := func(vlanMode *string, vlanID *int32) *spiderpoolv2beta1.SpiderMultusConfig {
		return &spiderpoolv2beta1.SpiderMultusConfig{
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.VlanCNI),
				VlanConfig: &spiderpoolv2beta1.SpiderVlanCniConfig{
					Master:   []string{"eth0"},
					VlanMode: vlanMode,
					VlanID:   vlanID,
				},
				ChainCNIJsonData: []string{},
			},
		}
	}

	It("defaults vlanMode to manual and vlanID to 0", func() {
		smc := newVlanSMC(nil, nil)

		mutateSpiderMultusConfig(logutils.IntoContext(context.Background(), zap.NewNop()), smc)

		Expect(smc.Spec.VlanConfig.VlanMode).NotTo(BeNil())
		Expect(*smc.Spec.VlanConfig.VlanMode).To(Equal(constant.VlanModeManual))
		Expect(smc.Spec.VlanConfig.VlanID).NotTo(BeNil())
		Expect(*smc.Spec.VlanConfig.VlanID).To(Equal(int32(0)))
		Expect(validateCNIConfig(smc)).To(BeNil())
	})

	It("requires vlanID when vlanMode is manual", func() {
		smc := newVlanSMC(ptr.To(constant.VlanModeManual), nil)

		err := validateCNIConfig(smc)

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("vlanId must not be nil when vlanMode is manual"))
	})

	It("forbids vlanID when vlanMode is auto", func() {
		smc := newVlanSMC(ptr.To(constant.VlanModeAuto), ptr.To(int32(100)))

		err := validateCNIConfig(smc)

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("vlanId must not be specified when vlanMode is auto"))
	})

	It("omits vlanId in generated vlan CNI config when vlanMode is auto", func() {
		smc := newVlanSMC(ptr.To(constant.VlanModeAuto), nil)
		mutateSpiderMultusConfig(logutils.IntoContext(context.Background(), zap.NewNop()), smc)

		conf := generateVlanCNIConf(false, smc.Spec)
		data, err := json.Marshal(conf)
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]interface{}
		Expect(json.Unmarshal(data, &decoded)).To(Succeed())
		Expect(decoded).To(HaveKeyWithValue("type", constant.VlanCNI))
		Expect(decoded).To(HaveKeyWithValue("master", "eth0"))
		Expect(decoded).To(HaveKeyWithValue("vlanMode", constant.VlanModeAuto))
		Expect(decoded).NotTo(HaveKey("vlanId"))
	})
})
