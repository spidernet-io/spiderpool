// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package multuscniconfig

import (
	"context"

	coordinator_cmd "github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"k8s.io/utils/ptr"
)

func mutateSpiderMultusConfig(ctx context.Context, smc *spiderpoolv2beta1.SpiderMultusConfig) {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate SpiderMultusConfig")

	// In the SpiderMultusConfig resource first creation, if we don't set `Spec.CniType` field, we need to set it to `custom`.
	// The kubernetes webhook is called before OpenAPI JSONSchema validation
	if (*smc).Spec.CniType == nil {
		(*smc).Spec.CniType = ptr.To(constant.CustomCNI)
	}
	switch *smc.Spec.CniType {
	case constant.MacvlanCNI:
		setMacvlanDefaultConfig(smc.Spec.MacvlanConfig)
	case constant.IPVlanCNI:
		setIPVlanDefaultConfig(smc.Spec.IPVlanConfig)
	case constant.SriovCNI:
		setSriovDefaultConfig(smc.Spec.SriovConfig)
	case constant.IBSriovCNI:
		setIBSriovDefaultConfig(smc.Spec.IbSriovConfig)
	case constant.IPoIBCNI:
		setIpoibDefaultConfig(smc.Spec.IpoibConfig)
	case constant.OvsCNI:
		setOvsDefaultConfig(smc.Spec.OvsConfig)
	case constant.CustomCNI:
		if smc.Spec.CustomCNIConfig == nil {
			smc.Spec.CustomCNIConfig = ptr.To("")
		}
	}

	// with custom CNI configuration, we don't need to add Coordinator configuration
	if *smc.Spec.CniType == constant.CustomCNI {
		smc.Spec.CoordinatorConfig = nil
		smc.Spec.EnableCoordinator = ptr.To(false)
	} else {
		smc.Spec.CoordinatorConfig = setCoordinatorDefaultConfig(smc.Spec.CoordinatorConfig)
	}

	if smc.Spec.ChainCNIJsonData == nil {
		smc.Spec.ChainCNIJsonData = []string{}
	}
}

func setMacvlanDefaultConfig(macvlanConfig *spiderpoolv2beta1.SpiderMacvlanCniConfig) {
	if macvlanConfig == nil {
		return
	}

	if macvlanConfig.VlanID == nil {
		macvlanConfig.VlanID = ptr.To(int32(0))
	}

	if macvlanConfig.Bond != nil {
		macvlanConfig.Bond = setBondDefaultConfig(macvlanConfig.Bond)
	}

	if macvlanConfig.SpiderpoolConfigPools == nil {
		macvlanConfig.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{},
			IPv6IPPool: []string{},
		}
	}
}

func setBondDefaultConfig(bond *spiderpoolv2beta1.BondConfig) *spiderpoolv2beta1.BondConfig {
	if bond.Options == nil {
		bond.Options = ptr.To("")
	}
	return bond
}

func setIPVlanDefaultConfig(ipvlanConfig *spiderpoolv2beta1.SpiderIPvlanCniConfig) {
	if ipvlanConfig == nil {
		return
	}

	if ipvlanConfig.VlanID == nil {
		ipvlanConfig.VlanID = ptr.To(int32(0))
	}

	if ipvlanConfig.Bond != nil {
		ipvlanConfig.Bond = setBondDefaultConfig(ipvlanConfig.Bond)
	}

	if ipvlanConfig.SpiderpoolConfigPools == nil {
		ipvlanConfig.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{},
			IPv6IPPool: []string{},
		}
	}
}

func setSriovDefaultConfig(sriovConfig *spiderpoolv2beta1.SpiderSRIOVCniConfig) {
	if sriovConfig == nil {
		return
	}

	if sriovConfig.VlanID == nil {
		sriovConfig.VlanID = ptr.To(int32(0))
	}

	if sriovConfig.MinTxRateMbps == nil {
		sriovConfig.MinTxRateMbps = ptr.To(0)
	}

	if sriovConfig.MaxTxRateMbps == nil {
		sriovConfig.MaxTxRateMbps = ptr.To(0)
	}

	if sriovConfig.SpiderpoolConfigPools == nil {
		sriovConfig.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{},
			IPv6IPPool: []string{},
		}
	}
}

func setIBSriovDefaultConfig(ibsriovConfig *spiderpoolv2beta1.SpiderIBSriovCniConfig) {
	if ibsriovConfig == nil {
		return
	}

	if ibsriovConfig.Pkey == nil {
		ibsriovConfig.Pkey = ptr.To("")
	}

	if ibsriovConfig.IbKubernetesEnabled == nil {
		ibsriovConfig.IbKubernetesEnabled = ptr.To(false)
	}

	if ibsriovConfig.RdmaIsolation == nil {
		ibsriovConfig.RdmaIsolation = ptr.To(true)
	}

	if ibsriovConfig.LinkState == nil {
		ibsriovConfig.LinkState = ptr.To("enable")
	}

	if ibsriovConfig.SpiderpoolConfigPools == nil {
		ibsriovConfig.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{},
			IPv6IPPool: []string{},
		}
	}
}

func setIpoibDefaultConfig(config *spiderpoolv2beta1.SpiderIpoibCniConfig) {
	if config == nil {
		return
	}
	if config.SpiderpoolConfigPools == nil {
		config.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{},
			IPv6IPPool: []string{},
		}
	}
}

func setOvsDefaultConfig(ovsConfig *spiderpoolv2beta1.SpiderOvsCniConfig) {
	if ovsConfig == nil {
		return
	}

	if ovsConfig.VlanTag == nil {
		ovsConfig.VlanTag = ptr.To(int32(0))
	}

	if ovsConfig.SpiderpoolConfigPools == nil {
		ovsConfig.SpiderpoolConfigPools = &spiderpoolv2beta1.SpiderpoolPools{
			IPv4IPPool: []string{},
			IPv6IPPool: []string{},
		}
	}
}

func setCoordinatorDefaultConfig(coordinator *spiderpoolv2beta1.CoordinatorSpec) *spiderpoolv2beta1.CoordinatorSpec {
	if coordinator == nil {
		return &spiderpoolv2beta1.CoordinatorSpec{
			Mode:               ptr.To(string(coordinator_cmd.ModeAuto)),
			HijackCIDR:         []string{},
			DetectGateway:      ptr.To(false),
			DetectIPConflict:   ptr.To(false),
			VethLinkAddress:    ptr.To(""),
			PodMACPrefix:       ptr.To(""),
			PodDefaultRouteNIC: ptr.To(""),
			HostRPFilter:       ptr.To(0),
			PodRPFilter:        ptr.To(0),
			TunePodRoutes:      ptr.To(true),
		}
	}

	if coordinator.Mode == nil {
		coordinator.Mode = ptr.To(string(coordinator_cmd.ModeAuto))
	}

	if coordinator.HijackCIDR == nil {
		coordinator.HijackCIDR = []string{}
	}

	if coordinator.DetectGateway == nil {
		coordinator.DetectGateway = ptr.To(false)
	}

	if coordinator.DetectIPConflict == nil {
		coordinator.DetectIPConflict = ptr.To(false)
	}

	if coordinator.PodMACPrefix == nil {
		coordinator.PodMACPrefix = ptr.To("")
	}

	if coordinator.PodDefaultRouteNIC == nil {
		coordinator.PodDefaultRouteNIC = ptr.To("")
	}

	if coordinator.TunePodRoutes == nil {
		coordinator.TunePodRoutes = ptr.To(false)
	}

	if coordinator.TxQueueLen == nil {
		coordinator.TxQueueLen = ptr.To(0)
	}

	if coordinator.PodRPFilter == nil {
		coordinator.PodRPFilter = ptr.To(0)
	}

	if coordinator.HostRPFilter == nil {
		coordinator.HostRPFilter = ptr.To(0)
	}

	return coordinator
}
