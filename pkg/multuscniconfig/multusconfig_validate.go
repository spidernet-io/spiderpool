// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var (
	cniTypeField         = field.NewPath("spec").Child("cniType")
	macvlanConfigField   = field.NewPath("spec").Child("macvlanConfig")
	ipvlanConfigField    = field.NewPath("spec").Child("ipvlanConfig")
	sriovConfigField     = field.NewPath("spec").Child("sriovConfig")
	customCniConfigField = field.NewPath("spec").Child("customCniTypeConfig")
)

func validateCNIConfig(multusConfig *spiderpoolv2beta1.SpiderMultusConfig) *field.Error {
	switch multusConfig.Spec.CniType {
	case MacVlanType:
		if multusConfig.Spec.MacvlanConfig == nil {
			return field.Required(macvlanConfigField, fmt.Sprintf("no %s specified", macvlanConfigField.String()))
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
		if multusConfig.Spec.MacvlanConfig != nil || multusConfig.Spec.IPVlanConfig != nil || multusConfig.Spec.CustomCNIConfig != nil {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", SriovType, sriovConfigField.String()))
		}

	case CustomType:
		// multusConfig.Spec.CustomCNIConfig can be empty

		if multusConfig.Spec.MacvlanConfig != nil || multusConfig.Spec.IPVlanConfig != nil || multusConfig.Spec.SriovConfig != nil {
			return field.Forbidden(cniTypeField, fmt.Sprintf("the cniType %s only supports %s, please remove other CNI configs", CustomType, customCniConfigField.String()))
		}
	}

	if multusConfig.Spec.CoordinatorConfig != nil {
		err := coordinatormanager.ValidateCoordinatorSpec(multusConfig.Spec.CoordinatorConfig.DeepCopy())
		if nil != err {
			return err
		}
	}

	return nil
}

func validateVlanCNIConfig(master []string, bond *spiderpoolv2beta1.BondConfig) error {
	if len(master) >= 2 {
		if bond == nil {
			return fmt.Errorf("the bond property is nil with multiple Interfaces")
		}
	}

	return nil
}
