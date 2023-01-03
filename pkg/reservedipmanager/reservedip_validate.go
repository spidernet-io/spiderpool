// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var (
	ipVersionField *field.Path = field.NewPath("spec").Child("ipVersion")
	ipsField       *field.Path = field.NewPath("spec").Child("ips")
)

func (rw *ReservedIPWebhook) validateCreateReservedIP(ctx context.Context, rIP *spiderpoolv1.SpiderReservedIP) field.ErrorList {
	if err := rw.validateReservedIPIPVersion(rIP.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := rw.validateReservedIPSpec(ctx, rIP); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (rw *ReservedIPWebhook) validateUpdateReservedIP(ctx context.Context, oldRIP, newRIP *spiderpoolv1.SpiderReservedIP) field.ErrorList {
	if err := validateReservedIPShouldNotBeChanged(oldRIP, newRIP); err != nil {
		return field.ErrorList{err}
	}

	if err := rw.validateReservedIPIPVersion(newRIP.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := rw.validateReservedIPSpec(ctx, newRIP); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func validateReservedIPShouldNotBeChanged(oldRIP, newRIP *spiderpoolv1.SpiderReservedIP) *field.Error {
	if newRIP.Spec.IPVersion != nil && oldRIP.Spec.IPVersion != nil &&
		*newRIP.Spec.IPVersion != *oldRIP.Spec.IPVersion {
		return field.Forbidden(
			ipVersionField,
			"is not changeable",
		)
	}

	return nil
}

func (rw *ReservedIPWebhook) validateReservedIPSpec(ctx context.Context, rIP *spiderpoolv1.SpiderReservedIP) *field.Error {
	return rw.validateReservedIPAvailableIP(ctx, *rIP.Spec.IPVersion, rIP)
}

func (rw *ReservedIPWebhook) validateReservedIPIPVersion(version *types.IPVersion) *field.Error {
	if version == nil {
		return field.Invalid(
			ipVersionField,
			version,
			"is not generated correctly, 'spec.ips' is empty or 'spec.ips[0]' may be invalid",
		)
	}

	if *version != constant.IPv4 && *version != constant.IPv6 {
		return field.NotSupported(
			ipVersionField,
			version,
			[]string{
				strconv.FormatInt(constant.IPv4, 10),
				strconv.FormatInt(constant.IPv6, 10),
			},
		)
	}

	if *version == constant.IPv4 && !rw.EnableIPv4 {
		return field.Forbidden(
			ipVersionField,
			"IPv4 is disabled",
		)
	}

	if *version == constant.IPv6 && !rw.EnableIPv6 {
		return field.Forbidden(
			ipVersionField,
			"IPv6 is disabled",
		)
	}

	return nil
}

func (rw *ReservedIPWebhook) validateReservedIPAvailableIP(ctx context.Context, version types.IPVersion, rIP *spiderpoolv1.SpiderReservedIP) *field.Error {
	if len(rIP.Spec.IPs) == 0 {
		return nil
	}

	newReservedIPs, err := spiderpoolip.ParseIPRanges(version, rIP.Spec.IPs)
	if err != nil {
		return field.Invalid(
			ipsField,
			rIP.Spec.IPs,
			err.Error(),
		)
	}

	var rIPList spiderpoolv1.SpiderReservedIPList
	if err := rw.List(ctx, &rIPList); err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to list ReservedIPs: %v", err))
	}

	for _, r := range rIPList.Items {
		if r.Name == rIP.Name || *r.Spec.IPVersion != version {
			continue
		}

		existReservedIPs, err := spiderpoolip.ParseIPRanges(version, r.Spec.IPs)
		if err != nil {
			return field.InternalError(ipsField, fmt.Errorf("failed to parse 'spec.ips':\n%v\n of the existing ReservedIP %s: %v", r.Spec.IPs, r.Name, err))
		}
		if len(spiderpoolip.IPsIntersectionSet(newReservedIPs, existReservedIPs)) > 0 {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("overlaps with the existing ReservedIP %s", r.Name),
			)
		}
	}

	return nil
}
