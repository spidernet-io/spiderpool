// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

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
	ipVersionField  *field.Path = field.NewPath("spec").Child("ipVersion")
	subnetField     *field.Path = field.NewPath("spec").Child("subnet")
	ipsField        *field.Path = field.NewPath("spec").Child("ips")
	excludeIPsField *field.Path = field.NewPath("spec").Child("excludeIPs")
	gatewayField    *field.Path = field.NewPath("spec").Child("gateway")
	routesField     *field.Path = field.NewPath("spec").Child("routes")
)

var (
	freeIPsField *field.Path = field.NewPath("status").Child("freeIPs")
)

func (im *ipPoolManager) validateCreateIPPool(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	var errs field.ErrorList
	if err := im.validateIPPoolSpec(ctx, ipPool); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (im *ipPoolManager) validateUpdateIPPool(ctx context.Context, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	if err := im.validateIPPoolShouldNotBeChanged(ctx, oldIPPool, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	if err := im.validateIPPoolSpec(ctx, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := im.validateIPPoolIPInUse(ctx, oldIPPool, newIPPool); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (im *ipPoolManager) validateIPPoolShouldNotBeChanged(ctx context.Context, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if *newIPPool.Spec.IPVersion != *oldIPPool.Spec.IPVersion {
		return field.Forbidden(
			ipVersionField,
			"is not changeable",
		)
	}

	if newIPPool.Spec.Subnet != oldIPPool.Spec.Subnet {
		return field.Forbidden(
			subnetField,
			"is not changeable",
		)
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolSpec(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if err := im.validateIPPoolIPVersion(ctx, ipPool.Spec.IPVersion); err != nil {
		return err
	}
	if err := im.validateIPPoolSubnet(ctx, *ipPool.Spec.IPVersion, ipPool.Name, ipPool.Spec.Subnet); err != nil {
		return err
	}
	if err := im.validateIPPoolAvailableIPs(ctx, ipPool); err != nil {
		return err
	}
	if err := im.validateIPPoolGateway(ctx, *ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.Gateway); err != nil {
		return err
	}
	if err := im.validateIPPoolRoutes(ctx, *ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.Routes); err != nil {
		return err
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolIPInUse(ctx context.Context, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if err := im.validateIPPoolIPs(ctx, *newIPPool.Spec.IPVersion, newIPPool.Spec.Subnet, newIPPool.Spec.IPs); err != nil {
		return err
	}
	if err := im.validateIPPoolExcludeIPs(ctx, *newIPPool.Spec.IPVersion, newIPPool.Spec.Subnet, newIPPool.Spec.ExcludeIPs); err != nil {
		return err
	}

	oldTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*oldIPPool.Spec.IPVersion, oldIPPool.Spec.IPs, oldIPPool.Spec.ExcludeIPs)
	newTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*newIPPool.Spec.IPVersion, newIPPool.Spec.IPs, newIPPool.Spec.ExcludeIPs)
	reducedIPs := spiderpoolip.IPsDiffSet(oldTotalIPs, newTotalIPs)

	for _, ip := range reducedIPs {
		if allocation, ok := newIPPool.Status.AllocatedIPs[ip.String()]; ok {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("remove an IP address %s that is being used by Pod %s/%s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ip.String(), allocation.Namespace, allocation.Pod),
			)
		}
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolIPVersion(ctx context.Context, version *types.IPVersion) *field.Error {
	if version == nil {
		return field.Invalid(
			ipVersionField,
			version,
			"is not generated correctly, 'spec.subnet' may be invalid",
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

func (im *ipPoolManager) validateIPPoolSubnet(ctx context.Context, version types.IPVersion, poolName, subnet string) *field.Error {
	ipPoolList, err := im.ListIPPools(ctx)
	if err != nil {
		return field.InternalError(subnetField, err)
	}

	for _, pool := range ipPoolList.Items {
		if pool.Name == poolName || *pool.Spec.IPVersion != version {
			continue
		}

		overlap, err := spiderpoolip.IsCIDROverlap(version, subnet, pool.Spec.Subnet)
		if err != nil {
			return field.Invalid(
				subnetField,
				subnet,
				err.Error(),
			)
		}

		if overlap {
			return field.Invalid(
				subnetField,
				subnet,
				fmt.Sprintf("overlap with IPPool %s which 'spec.subnet' is %s", pool.Name, pool.Spec.Subnet),
			)
		}
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolAvailableIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if err := im.validateIPPoolIPs(ctx, *ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.IPs); err != nil {
		return err
	}
	if err := im.validateIPPoolExcludeIPs(ctx, *ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.ExcludeIPs); err != nil {
		return err
	}

	ipPoolList, err := im.ListIPPools(ctx)
	if err != nil {
		return field.InternalError(ipsField, err)
	}

	newIPs, _ := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	for _, pool := range ipPoolList.Items {
		if pool.Name == ipPool.Name || pool.Spec.Subnet != ipPool.Spec.Subnet {
			continue
		}

		existIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
		if err != nil {
			return field.InternalError(ipsField, err)
		}
		if len(newIPs) > len(spiderpoolip.IPsDiffSet(newIPs, existIPs)) {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("overlap with IPPool %s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", pool.Name),
			)
		}
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolIPs(ctx context.Context, version types.IPVersion, subnet string, ips []string) *field.Error {
	if len(ips) == 0 {
		return field.Required(
			ipsField,
			"requires at least one item",
		)
	}

	for i, r := range ips {
		if err := ValidateContainsIPRange(ctx, ipsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolExcludeIPs(ctx context.Context, version types.IPVersion, subnet string, excludeIPs []string) *field.Error {
	for i, r := range excludeIPs {
		if err := ValidateContainsIPRange(ctx, excludeIPsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolGateway(ctx context.Context, version types.IPVersion, subnet string, gateway *string) *field.Error {
	if gateway != nil {
		return ValidateContainsIP(ctx, gatewayField, version, subnet, *gateway)
	}

	return nil
}

func (im *ipPoolManager) validateIPPoolRoutes(ctx context.Context, version types.IPVersion, subnet string, routes []spiderpoolv1.Route) *field.Error {
	for i, r := range routes {
		if err := spiderpoolip.IsCIDR(version, r.Dst); err != nil {
			return field.Invalid(
				routesField.Index(i).Child("dst"),
				r.Dst,
				err.Error(),
			)
		}

		if err := ValidateContainsIP(ctx, routesField.Index(i).Child("gw"), version, subnet, r.Gw); err != nil {
			return err
		}
	}

	return nil
}

func ValidateContainsIPRange(ctx context.Context, fieldPath *field.Path, version types.IPVersion, subnet string, ipRange string) *field.Error {
	contains, err := spiderpoolip.ContainsIPRange(version, subnet, ipRange)
	if err != nil {
		return field.Invalid(
			fieldPath,
			ipRange,
			err.Error(),
		)
	}

	if !contains {
		return field.Invalid(
			fieldPath,
			ipRange,
			fmt.Sprintf("not pertains to the 'spec.subnet' %s of IPPool", subnet),
		)
	}

	return nil
}

func ValidateContainsIP(ctx context.Context, fieldPath *field.Path, version types.IPVersion, subnet string, ip string) *field.Error {
	contains, err := spiderpoolip.ContainsIP(version, subnet, ip)
	if err != nil {
		return field.Invalid(
			fieldPath,
			ip,
			err.Error(),
		)
	}

	if !contains {
		return field.Invalid(
			fieldPath,
			ip,
			fmt.Sprintf("not pertains to the 'spec.subnet' %s of IPPool", subnet),
		)
	}

	return nil
}
