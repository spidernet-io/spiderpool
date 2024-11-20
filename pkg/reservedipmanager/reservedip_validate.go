// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var (
	ipVersionField *field.Path = field.NewPath("spec").Child("ipVersion")
	ipsField       *field.Path = field.NewPath("spec").Child("ips")
)

func (rw *ReservedIPWebhook) validateCreateReservedIP(ctx context.Context, rIP *spiderpoolv2beta1.SpiderReservedIP) field.ErrorList {
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

func (rw *ReservedIPWebhook) validateUpdateReservedIP(ctx context.Context, oldRIP, newRIP *spiderpoolv2beta1.SpiderReservedIP) field.ErrorList {
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

func validateReservedIPShouldNotBeChanged(oldRIP, newRIP *spiderpoolv2beta1.SpiderReservedIP) *field.Error {
	if newRIP.Spec.IPVersion != nil && oldRIP.Spec.IPVersion != nil &&
		*newRIP.Spec.IPVersion != *oldRIP.Spec.IPVersion {
		return field.Forbidden(
			ipVersionField,
			"is not changeable",
		)
	}

	return nil
}

func (rw *ReservedIPWebhook) validateReservedIPSpec(ctx context.Context, rIP *spiderpoolv2beta1.SpiderReservedIP) *field.Error {
	return rw.validateReservedIPs(ctx, *rIP.Spec.IPVersion, rIP.Spec.IPs)
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

func (rw *ReservedIPWebhook) validateReservedIPs(ctx context.Context, version types.IPVersion, ips []string) *field.Error {
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
