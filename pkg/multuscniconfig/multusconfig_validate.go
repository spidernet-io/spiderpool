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
	cniTypeField         = field.NewPath("spec").Child("cniType")
	macvlanConfigField   = field.NewPath("spec").Child("macvlanConfig")
	ipvlanConfigField    = field.NewPath("spec").Child("ipvlanConfig")
	sriovConfigField     = field.NewPath("spec").Child("sriovConfig")
	customCniConfigField = field.NewPath("spec").Child("customCniTypeConfig")
	annotationField      = field.NewPath("metadata").Child("annotations")
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

func validateCNIConfig(multusConfig *spiderpoolv2beta1.SpiderMultusConfig) *field.Error {
	switch multusConfig.Spec.CniType {
	case MacVlanType:
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

		if multusConfig.Spec.IPVlanConfig != nil || multusConfig.Spec.SriovConfig != nil || multusConfig.Spec.CustomCNIConfig != nil {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", MacVlanType, macvlanConfigField.String()))
		}

	case IpVlanType:
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

		if multusConfig.Spec.MacvlanConfig != nil || multusConfig.Spec.SriovConfig != nil || multusConfig.Spec.CustomCNIConfig != nil {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", IpVlanType, ipvlanConfigField.String()))
		}

	case SriovType:
		if multusConfig.Spec.SriovConfig == nil {
			return field.Required(sriovConfigField, fmt.Sprintf("no %s specified", sriovConfigField.String()))
		}

		if multusConfig.Spec.SriovConfig.VlanID != nil {
			if err := validateVlanId(*multusConfig.Spec.SriovConfig.VlanID); err != nil {
				return field.Invalid(sriovConfigField, *multusConfig.Spec.SriovConfig.VlanID, err.Error())
			}
		}

		if multusConfig.Spec.SriovConfig.ResourceName == "" {
			return field.Required(sriovConfigField, fmt.Sprintf("no %s specified", sriovConfigField.Key("resourceName")))
		}

		if multusConfig.Spec.MacvlanConfig != nil || multusConfig.Spec.IPVlanConfig != nil || multusConfig.Spec.CustomCNIConfig != nil {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", SriovType, sriovConfigField.String()))
		}

	case CustomType:
		// multusConfig.Spec.CustomCNIConfig can be empty
		if multusConfig.Spec.MacvlanConfig != nil || multusConfig.Spec.IPVlanConfig != nil || multusConfig.Spec.SriovConfig != nil {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", CustomType, customCniConfigField.String()))
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
