// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"

	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var (
	podCIDRTypeField     *field.Path = field.NewPath("spec").Child("podCIDRType")
	extraCIDRField       *field.Path = field.NewPath("spec").Child("extraCIDR")
	podMACPrefixField    *field.Path = field.NewPath("spec").Child("podMACPrefix")
	podRPFilterField     *field.Path = field.NewPath("spec").Child("podRPFilter")
	txQueueLenField      *field.Path = field.NewPath("spec").Child("txQueueLen")
	vethLinkAddressField *field.Path = field.NewPath("spec").Child("vethLinkAddress")
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

	if spec.TxQueueLen != nil && *spec.TxQueueLen < 0 {
		return field.Invalid(txQueueLenField, *spec.TxQueueLen, "txQueueLen can't be less than 0")
	}

	if requireOptionalType && spec.PodRPFilter == nil {
		return field.NotSupported(
			podRPFilterField,
			nil,
			[]string{"0", "1", "2"},
		)
	}
	if spec.PodRPFilter != nil {
		if err := validateCoordinatorPodRPFilter(spec.PodRPFilter); err != nil {
			return err
		}
	}

	if spec.VethLinkAddress != nil && *spec.VethLinkAddress != "" {
		_, err := netip.ParseAddr(*spec.VethLinkAddress)
		if err != nil {
			return field.Invalid(vethLinkAddressField, *spec.VethLinkAddress, "vethLinkAddress is an invalid IP address")
		}
	}

	return nil
}

func validateCoordinatorPodCIDRType(t string) *field.Error {
	if !slices.Contains(SupportedPodCIDRType, t) {
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
		nPrefix, err := ip.ParseIPOrCIDR(cidr)
		if err != nil {
			return field.Invalid(
				extraCIDRField.Index(i),
				cidr,
				err.Error(),
			)
		}
		cidrs[i] = nPrefix.String()
	}
	return nil
}

func validateCoordinatorPodMACPrefix(prefix *string) *field.Error {
	if prefix == nil || *prefix == "" {
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
		return field.Invalid(podMACPrefixField, *prefix, "not a unicast MAC: the lowest bit of the first byte must be 0")
	}

	return nil
}

func validateCoordinatorPodRPFilter(f *int) *field.Error {
	if *f >= 0 {
		if *f != 0 && *f != 1 && *f != 2 {
			return field.NotSupported(
				podRPFilterField,
				*f,
				[]string{"0", "1", "2"},
			)
		}
	}
	return nil
}
