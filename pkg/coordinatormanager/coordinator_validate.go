// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var (
	podCIDRTypeField  *field.Path = field.NewPath("spec").Child("podCIDRType")
	extraCIDRField    *field.Path = field.NewPath("spec").Child("extraCIDR")
	podMACPrefixField *field.Path = field.NewPath("spec").Child("podMACPrefix")
	hostRPFilterField *field.Path = field.NewPath("spec").Child("hostRPFilter")
)

func validateCreateCoordinator(coord *spiderpoolv2beta1.SpiderCoordinator) field.ErrorList {
	var errs field.ErrorList
	if err := ValidateCoordinatorSpec(coord.Spec.DeepCopy(), true); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func validateUpdateCoordinator(oldCoord, newCoord *spiderpoolv2beta1.SpiderCoordinator) field.ErrorList {
	var errs field.ErrorList
	if err := ValidateCoordinatorSpec(newCoord.Spec.DeepCopy(), true); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func ValidateCoordinatorSpec(spec *spiderpoolv2beta1.CoordinatorSpec, requireOptionalType bool) *field.Error {

	if requireOptionalType && spec.PodCIDRType == nil {
		return field.NotSupported(
			podCIDRTypeField,
			"",
			SupportedPodCIDRType,
		)
	}
	if spec.PodCIDRType != nil {
		if err := validateCoordinatorPodCIDRType(*spec.PodCIDRType); err != nil {
			return err
		}
	}

	if err := validateCoordinatorExtraCIDR(spec.HijackCIDR); err != nil {
		return err
	}
	if err := validateCoordinatorPodMACPrefix(spec.PodMACPrefix); err != nil {
		return err
	}

	if requireOptionalType && spec.HostRPFilter == nil {
		return field.NotSupported(
			hostRPFilterField,
			nil,
			[]string{"0", "1", "2"},
		)
	}
	if spec.HostRPFilter != nil {
		if err := validateCoordinatorhostRPFilter(spec.HostRPFilter); err != nil {
			return err
		}
	}

	return nil
}

func validateCoordinatorPodCIDRType(t string) *field.Error {
	if t != cluster && t != calico && t != cilium {
		return field.NotSupported(
			podCIDRTypeField,
			t,
			SupportedPodCIDRType,
		)
	}

	return nil
}

func validateCoordinatorExtraCIDR(cidrs []string) *field.Error {
	if len(cidrs) == 0 {
		return nil
	}

	for i, cidr := range cidrs {
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return field.Invalid(
				extraCIDRField.Index(i),
				cidr,
				err.Error(),
			)
		}
	}

	return nil
}

func validateCoordinatorPodMACPrefix(prefix *string) *field.Error {
	if prefix == nil {
		return nil
	}

	if *prefix == "" {
		return nil
	}

	errInvalid := field.Invalid(podMACPrefixField, *prefix, "not a valid MAC prefix")
	parts := strings.Split(*prefix, ":")
	if len(parts) != 2 {
		return errInvalid
	}

	if len(parts[0]) != 2 || len(parts[1]) != 2 {
		return errInvalid
	}

	fb, err := strconv.ParseInt(parts[0], 16, 0)
	if err != nil {
		return errInvalid
	}
	_, err = strconv.ParseInt(parts[1], 16, 0)
	if err != nil {
		return errInvalid
	}

	bb := fmt.Sprintf("%08b", fb)
	if string(bb[7]) != "0" {
		return field.Invalid(podMACPrefixField, *prefix, "not a unicast MAC")
	}

	return nil
}

func validateCoordinatorhostRPFilter(f *int) *field.Error {
	if *f != 0 && *f != 1 && *f != 2 {
		return field.NotSupported(
			hostRPFilterField,
			*f,
			[]string{"0", "1", "2"},
		)
	}

	return nil
}
