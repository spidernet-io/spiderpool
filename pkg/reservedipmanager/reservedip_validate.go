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

func (rm *reservedIPManager) validateCreateReservedIP(ctx context.Context, rIP *spiderpoolv1.SpiderReservedIP) field.ErrorList {
	var errs field.ErrorList
	if err := rm.validateReservedIPSpec(ctx, rIP); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (rm *reservedIPManager) validateUpdateReservedIP(ctx context.Context, oldRIP, newRIP *spiderpoolv1.SpiderReservedIP) field.ErrorList {
	if err := rm.validateReservedIPShouldNotBeChanged(ctx, oldRIP, newRIP); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := rm.validateReservedIPSpec(ctx, newRIP); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (rm *reservedIPManager) validateReservedIPShouldNotBeChanged(ctx context.Context, oldRIP, newRIP *spiderpoolv1.SpiderReservedIP) *field.Error {
	if *newRIP.Spec.IPVersion != *oldRIP.Spec.IPVersion {
		return field.Forbidden(
			ipVersionField,
			"is not changeable",
		)
	}

	return nil
}

func (rm *reservedIPManager) validateReservedIPSpec(ctx context.Context, rIP *spiderpoolv1.SpiderReservedIP) *field.Error {
	if err := rm.validateReservedIPIPVersion(ctx, rIP.Spec.IPVersion); err != nil {
		return err
	}
	if err := rm.validateReservedIPAvailableIP(ctx, *rIP.Spec.IPVersion, rIP); err != nil {
		return err
	}

	return nil
}

func (rm *reservedIPManager) validateReservedIPIPVersion(ctx context.Context, version *types.IPVersion) *field.Error {
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
			[]string{strconv.FormatInt(constant.IPv4, 10),
				strconv.FormatInt(constant.IPv6, 10),
			},
		)
	}

	return nil
}

func (rm *reservedIPManager) validateReservedIPAvailableIP(ctx context.Context, version types.IPVersion, rIP *spiderpoolv1.SpiderReservedIP) *field.Error {
	if err := rm.validateReservedIPIPs(ctx, version, rIP.Spec.IPs); err != nil {
		return err
	}

	rIPList, err := rm.ListReservedIPs(ctx)
	if err != nil {
		return field.InternalError(ipsField, err)
	}

	newReservedIPs, _ := spiderpoolip.ParseIPRanges(version, rIP.Spec.IPs)
	for _, r := range rIPList.Items {
		if r.Name == rIP.Name || *r.Spec.IPVersion != version {
			continue
		}

		existReservedIPs, err := spiderpoolip.ParseIPRanges(version, r.Spec.IPs)
		if err != nil {
			return field.InternalError(ipsField, err)
		}
		if len(newReservedIPs) > len(spiderpoolip.IPsDiffSet(newReservedIPs, existReservedIPs)) {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("overlaps with ReservedIP %s", r.Name),
			)
		}
	}

	return nil
}

func (rm *reservedIPManager) validateReservedIPIPs(ctx context.Context, version types.IPVersion, ips []string) *field.Error {
	if len(ips) == 0 {
		return field.Required(
			ipsField,
			"requires at least one item",
		)
	}

	for i, r := range ips {
		if err := spiderpoolip.IsIPRange(version, r); err != nil {
			return field.Invalid(
				ipsField.Index(i),
				ips[i],
				err.Error(),
			)
		}
	}

	return nil
}
