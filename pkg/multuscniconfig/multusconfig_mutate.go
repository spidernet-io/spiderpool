// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package multuscniconfig

import (
	"context"

	"k8s.io/utils/pointer"

	coordinator_cmd "github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func mutateSpiderMultusConfig(ctx context.Context, smc *spiderpoolv2beta1.SpiderMultusConfig) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate SpiderMultusConfig")

	switch smc.Spec.CniType {
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
			smc.Spec.CustomCNIConfig = pointer.String("")
		}
	}

	smc.Spec.CoordinatorConfig = setCoordinatorDefaultConfig(smc.Spec.CoordinatorConfig)
	return nil
}

func setMacvlanDefaultConfig(macvlanConfig *spiderpoolv2beta1.SpiderMacvlanCniConfig) {
	if macvlanConfig == nil {
		return
	}

	if macvlanConfig.VlanID == nil {
		macvlanConfig.VlanID = pointer.Int32(0)
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
		bond.Options = pointer.String("")
	}
	return bond
}

func setIPVlanDefaultConfig(ipvlanConfig *spiderpoolv2beta1.SpiderIPvlanCniConfig) {
	if ipvlanConfig == nil {
		return
	}

	if ipvlanConfig.VlanID == nil {
		ipvlanConfig.VlanID = pointer.Int32(0)
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
		sriovConfig.VlanID = pointer.Int32(0)
	}

	if sriovConfig.MinTxRateMbps == nil {
		sriovConfig.MinTxRateMbps = pointer.Int(0)
	}

	if sriovConfig.MaxTxRateMbps == nil {
		sriovConfig.MaxTxRateMbps = pointer.Int(0)
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
		ibsriovConfig.Pkey = pointer.String("")
	}

	if ibsriovConfig.IbKubernetesEnabled == nil {
		ibsriovConfig.IbKubernetesEnabled = pointer.Bool(false)
	}

	if ibsriovConfig.RdmaIsolation == nil {
		ibsriovConfig.RdmaIsolation = pointer.Bool(true)
	}

	if ibsriovConfig.LinkState == nil {
		ibsriovConfig.LinkState = pointer.String("enable")
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
		ovsConfig.VlanTag = pointer.Int32(0)
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
			Mode:               pointer.String(string(coordinator_cmd.ModeAuto)),
			HijackCIDR:         []string{},
			DetectGateway:      pointer.Bool(false),
			DetectIPConflict:   pointer.Bool(false),
			PodMACPrefix:       pointer.String(""),
			PodDefaultRouteNIC: pointer.String(""),
			TunePodRoutes:      pointer.Bool(true),
		}
	}

	if coordinator.Mode == nil {
		coordinator.Mode = pointer.String(string(coordinator_cmd.ModeAuto))
	}

	if coordinator.HijackCIDR == nil {
		coordinator.HijackCIDR = []string{}
	}

	if coordinator.DetectGateway == nil {
		coordinator.DetectGateway = pointer.Bool(false)
	}

	if coordinator.DetectIPConflict == nil {
		coordinator.DetectIPConflict = pointer.Bool(false)
	}

	if coordinator.PodMACPrefix == nil {
		coordinator.PodMACPrefix = pointer.String("")
	}

	if coordinator.PodDefaultRouteNIC == nil {
		coordinator.PodDefaultRouteNIC = pointer.String("")
	}

	if coordinator.TunePodRoutes == nil {
		coordinator.TunePodRoutes = pointer.Bool(false)
	}

	return coordinator
}
