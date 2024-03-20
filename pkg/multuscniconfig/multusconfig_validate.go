// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"encoding/json"
	"fmt"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"

	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var (
	cniTypeField          = field.NewPath("spec").Child("cniType")
	macvlanConfigField    = field.NewPath("spec").Child("macvlanConfig")
	ipvlanConfigField     = field.NewPath("spec").Child("ipvlanConfig")
	sriovConfigField      = field.NewPath("spec").Child("sriovConfig")
	ibsriovConfigField    = field.NewPath("spec").Child("ibsriovConfig")
	ipoibConfigField      = field.NewPath("spec").Child("ipoibConfig")
	ovsConfigField        = field.NewPath("spec").Child("ovsConfig")
	hostdeviceConfigField = field.NewPath("spec").Child("hostdeviceConfig")
	customCniConfigField  = field.NewPath("spec").Child("customCniTypeConfig")
	annotationField       = field.NewPath("metadata").Child("annotations")
)

func validate(oldMultusConfig, multusConfig *spiderpoolv2beta1.SpiderMultusConfig) *field.Error {
	if oldMultusConfig != nil {
		err := validateCustomAnnoNameShouldNotBeChangeable(oldMultusConfig, multusConfig)
		if nil != err {
			return err
		}
	}

	err := validateAnnotation(multusConfig)
	if nil != err {
		return err
	}

	err = validateCNIConfig(multusConfig)
	if nil != err {
		return err
	}

	return nil
}

func checkExistedConfig(spec *spiderpoolv2beta1.MultusCNIConfigSpec, exclude string) bool {
	if exclude != constant.MacvlanCNI && spec.MacvlanConfig != nil {
		return true
	}
	if exclude != constant.IPVlanCNI && spec.IPVlanConfig != nil {
		return true
	}
	if exclude != constant.OvsCNI && spec.OvsConfig != nil {
		return true
	}
	if exclude != constant.HostDeviceCNI && spec.HostDeviceConfig != nil {
		return true
	}
	if exclude != constant.SriovCNI && spec.SriovConfig != nil {
		return true
	}
	if exclude != constant.IBSriovCNI && spec.IbSriovConfig != nil {
		return true
	}
	if exclude != constant.IPoIBCNI && spec.IpoibConfig != nil {
		return true
	}
	if exclude != constant.CustomCNI && spec.CustomCNIConfig != nil {
		return true
	}

	return false
}

func validateCNIConfig(multusConfig *spiderpoolv2beta1.SpiderMultusConfig) *field.Error {
	// with Kubernetes OpenAPI validation and Mutating Webhook, multusConfSpec.CniType must not be nil and default to "custom"
	switch *multusConfig.Spec.CniType {
	case constant.MacvlanCNI:
		if multusConfig.Spec.MacvlanConfig == nil {
			return field.Required(macvlanConfigField, fmt.Sprintf("no %s specified", macvlanConfigField.String()))
		}

		if multusConfig.Spec.MacvlanConfig.VlanID != nil {
			if err := validateVlanId(*multusConfig.Spec.MacvlanConfig.VlanID); err != nil {
				return field.Invalid(macvlanConfigField, *multusConfig.Spec.MacvlanConfig.VlanID, err.Error())
			}
		}

		if err := validateVlanCNIConfig(multusConfig.Spec.MacvlanConfig.Master, multusConfig.Spec.MacvlanConfig.Bond); err != nil {
			return field.Invalid(macvlanConfigField, *multusConfig.Spec.MacvlanConfig, err.Error())
		}

		if checkExistedConfig(&(multusConfig.Spec), constant.MacvlanCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, macvlanConfigField.String()))
		}

	case constant.IPVlanCNI:
		if multusConfig.Spec.IPVlanConfig == nil {
			return field.Required(ipvlanConfigField, fmt.Sprintf("no %s specified", ipvlanConfigField.String()))
		}

		if multusConfig.Spec.IPVlanConfig.VlanID != nil {
			if err := validateVlanId(*multusConfig.Spec.IPVlanConfig.VlanID); err != nil {
				return field.Invalid(ipvlanConfigField, *multusConfig.Spec.IPVlanConfig.VlanID, err.Error())
			}
		}

		if err := validateVlanCNIConfig(multusConfig.Spec.IPVlanConfig.Master, multusConfig.Spec.IPVlanConfig.Bond); err != nil {
			return field.Invalid(ipvlanConfigField, *multusConfig.Spec.IPVlanConfig, err.Error())
		}

		if checkExistedConfig(&(multusConfig.Spec), constant.IPVlanCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, ipvlanConfigField.String()))
		}

	case constant.SriovCNI:
		if multusConfig.Spec.SriovConfig == nil {
			return field.Required(sriovConfigField, fmt.Sprintf("no %s specified", sriovConfigField.String()))
		}

		if multusConfig.Spec.SriovConfig.VlanID != nil {
			if err := validateVlanId(*multusConfig.Spec.SriovConfig.VlanID); err != nil {
				return field.Invalid(sriovConfigField, *multusConfig.Spec.SriovConfig.VlanID, err.Error())
			}
		}

		if multusConfig.Spec.SriovConfig.MinTxRateMbps != nil && multusConfig.Spec.SriovConfig.MaxTxRateMbps != nil {
			if *multusConfig.Spec.SriovConfig.MinTxRateMbps > *multusConfig.Spec.SriovConfig.MaxTxRateMbps {
				return field.Invalid(sriovConfigField, *multusConfig.Spec.SriovConfig.MinTxRateMbps, "minTxRateMbps must be less than maxTxRateMbps")
			}
		}

		if multusConfig.Spec.SriovConfig.ResourceName == "" {
			return field.Required(sriovConfigField, fmt.Sprintf("no %s specified", sriovConfigField.Key("resourceName")))
		}

		if checkExistedConfig(&(multusConfig.Spec), constant.SriovCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, sriovConfigField.String()))
		}

	case constant.IBSriovCNI:
		if multusConfig.Spec.IbSriovConfig == nil {
			return field.Required(ibsriovConfigField, fmt.Sprintf("no %s specified", ibsriovConfigField.String()))
		}

		if multusConfig.Spec.IbSriovConfig.ResourceName == "" {
			return field.Required(ibsriovConfigField, fmt.Sprintf("no %s specified", ibsriovConfigField.Key("resourceName")))
		}

		if checkExistedConfig(&(multusConfig.Spec), constant.IBSriovCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, sriovConfigField.String()))
		}

	case constant.IPoIBCNI:
		if multusConfig.Spec.IpoibConfig == nil {
			return field.Required(ipoibConfigField, fmt.Sprintf("no %s specified", ipoibConfigField.String()))
		}

		if len(multusConfig.Spec.IpoibConfig.Master) == 0 {
			return field.Required(ipoibConfigField, fmt.Sprintf("no %s specified", ipoibConfigField.Key("master")))
		}

		if checkExistedConfig(&(multusConfig.Spec), constant.IPoIBCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, sriovConfigField.String()))
		}

	case constant.OvsCNI:
		if multusConfig.Spec.OvsConfig == nil {
			return field.Required(ovsConfigField, fmt.Sprintf("no %s specified", ovsConfigField.String()))
		}

		if multusConfig.Spec.OvsConfig.VlanTag != nil {
			if err := validateVlanId(*multusConfig.Spec.OvsConfig.VlanTag); err != nil {
				return field.Invalid(ovsConfigField, *multusConfig.Spec.OvsConfig.VlanTag, err.Error())
			}
		}

		for idx, trunk := range multusConfig.Spec.OvsConfig.Trunk {
			if trunk.MinID != nil {
				if *trunk.MinID > 4094 {
					return field.Invalid(ovsConfigField, multusConfig.Spec.OvsConfig.Trunk[idx], "incorrect trunk minID parameter")
				}
			}
			if trunk.MaxID != nil {
				if *trunk.MaxID > 4094 {
					return field.Invalid(ovsConfigField, multusConfig.Spec.OvsConfig.Trunk[idx], "incorrect trunk maxID parameter")
				}
				if *trunk.MaxID < *trunk.MinID {
					return field.Invalid(ovsConfigField, multusConfig.Spec.OvsConfig.Trunk[idx], "minID is greater than maxID in trunk parameter")
				}
			}

			if trunk.ID != nil {
				if *trunk.ID > 4096 {
					return field.Invalid(ovsConfigField, multusConfig.Spec.OvsConfig.Trunk[idx], "incorrect trunk id parameter")
				}
			}
		}

		if checkExistedConfig(&(multusConfig.Spec), constant.OvsCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, sriovConfigField.String()))
		}

	case constant.HostDeviceCNI:
		if multusConfig.Spec.HostDeviceConfig.Device == "" && multusConfig.Spec.HostDeviceConfig.HWAddr == "" &&
			multusConfig.Spec.HostDeviceConfig.KernelPath == "" && multusConfig.Spec.HostDeviceConfig.PCIAddr == "" {
			return field.Required(hostdeviceConfigField, `specify either "deviceName", "hwaddr", "kernelpath" or "pciAddr"`)
		}
		if checkExistedConfig(&(multusConfig.Spec), constant.HostDeviceCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, hostdeviceConfigField.String()))
		}

	case constant.CustomCNI:
		// multusConfig.Spec.CustomCNIConfig can be empty
		if checkExistedConfig(&(multusConfig.Spec), constant.CustomCNI) {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", *multusConfig.Spec.CniType, customCniConfigField.String()))
		}

		if multusConfig.Spec.CustomCNIConfig != nil && *multusConfig.Spec.CustomCNIConfig != "" {
			if !json.Valid([]byte(*multusConfig.Spec.CustomCNIConfig)) {
				return field.Forbidden(customCniConfigField, "customCniConfig isn't a valid JSON encoding")
			}
		}
	}

	if multusConfig.Spec.CoordinatorConfig != nil {
		err := coordinatormanager.ValidateCoordinatorSpec(multusConfig.Spec.CoordinatorConfig.DeepCopy(), false)
		if nil != err {
			return err
		}
	}

	return nil
}

func validateVlanCNIConfig(master []string, bond *spiderpoolv2beta1.BondConfig) error {
	if len(master) == 0 {
		return fmt.Errorf("master can't be empty")
	} else if len(master) >= 2 {
		if bond == nil {
			return fmt.Errorf("the bond property is nil with multiple Interfaces")
		}
		if bond.Name == "" {
			return fmt.Errorf("bond name can't be empty")
		}
	}

	return nil
}

func validateVlanId(vlanId int32) error {
	if vlanId < 0 || vlanId > 4094 {
		return fmt.Errorf("invalid vlanId %v, please make sure vlanId in range [0,4094]", vlanId)
	}
	return nil
}

func validateAnnotation(multusConfig *spiderpoolv2beta1.SpiderMultusConfig) *field.Error {
	// validate the custom net-attach-def resource name
	customMultusName, ok := multusConfig.Annotations[constant.AnnoNetAttachConfName]
	if ok && customMultusName == "" {
		return field.Invalid(annotationField, multusConfig.Annotations, "invalid custom net-attach-def resource empty name")
	}
	if len(customMultusName) > k8svalidation.DNS1123SubdomainMaxLength {
		return field.Invalid(annotationField, multusConfig.Annotations,
			fmt.Sprintf("the custom net-attach-def resource name must be no more than %d characters", k8svalidation.DNS1123SubdomainMaxLength))
	}

	// validate the custom net-attach-def CNI version
	cniVersion, ok := multusConfig.Annotations[constant.AnnoMultusConfigCNIVersion]
	if ok && !slices.Contains(cmd.SupportCNIVersions, cniVersion) {
		return field.Invalid(annotationField, multusConfig.Annotations, fmt.Sprintf("unsupported CNI version %s", cniVersion))
	}
	return nil
}

func validateCustomAnnoNameShouldNotBeChangeable(oldMultusConfig, newMultusConfig *spiderpoolv2beta1.SpiderMultusConfig) *field.Error {
	oldCustomMultusName, oldOK := oldMultusConfig.Annotations[constant.AnnoNetAttachConfName]
	newCustomMultusName, newOK := newMultusConfig.Annotations[constant.AnnoNetAttachConfName]

	if (oldOK && newOK) && oldCustomMultusName != newCustomMultusName {
		return field.Invalid(annotationField, oldMultusConfig.Annotations[constant.AnnoNetAttachConfName],
			fmt.Sprintf("it's unsupported to change customized Multus net-attach-def resource name from '%s' to '%s'", oldCustomMultusName, newCustomMultusName))
	}

	if !oldOK && newOK {
		return field.Invalid(annotationField, newMultusConfig.Annotations[constant.AnnoNetAttachConfName],
			fmt.Sprintf("it's unsupported to changed Multus net-attach-def '%s' to customized name '%s'", oldMultusConfig.Name, newCustomMultusName))
	}

	return nil
}
