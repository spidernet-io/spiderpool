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

func (iw *IPPoolWebhook) validateCreateIPPool(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	if err := iw.validateIPPoolIPVersion(ipPool.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	if err := iw.validateIPPoolSubnet(ctx, ipPool); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := iw.validateIPPoolSpec(ctx, ipPool); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (iw *IPPoolWebhook) validateUpdateIPPool(ctx context.Context, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	if err := validateIPPoolShouldNotBeChanged(oldIPPool, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	if err := iw.validateIPPoolIPVersion(newIPPool.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	if err := iw.validateIPPoolSpec(ctx, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := validateIPPoolIPInUse(newIPPool); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func validateIPPoolShouldNotBeChanged(oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if newIPPool.Spec.IPVersion != nil && oldIPPool.Spec.IPVersion != nil &&
		*newIPPool.Spec.IPVersion != *oldIPPool.Spec.IPVersion {
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

func (iw *IPPoolWebhook) validateIPPoolSpec(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if err := iw.validateIPPoolAvailableIPs(ctx, ipPool); err != nil {
		return err
	}
	if err := validateIPPoolGateway(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.Gateway); err != nil {
		return err
	}

	return validateIPPoolRoutes(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.Routes)
}

func validateIPPoolIPInUse(ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	totalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %v", ipPool.Name, err))
	}

	totalIPsMap := map[string]bool{}
	for _, ip := range totalIPs {
		totalIPsMap[ip.String()] = true
	}

	for ip, allocation := range ipPool.Status.AllocatedIPs {
		if _, ok := totalIPsMap[ip]; !ok {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("remove an IP address %s that is being used by Pod %s/%s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ip, allocation.Namespace, allocation.Pod),
			)
		}
	}

	return nil
}

func (iw *IPPoolWebhook) validateIPPoolIPVersion(version *types.IPVersion) *field.Error {
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

	if *version == constant.IPv4 && !iw.EnableIPv4 {
		return field.Forbidden(
			ipVersionField,
			"IPv4 is disabled",
		)
	}

	if *version == constant.IPv6 && !iw.EnableIPv6 {
		return field.Forbidden(
			ipVersionField,
			"IPv6 is disabled",
		)
	}

	return nil
}

func (iw *IPPoolWebhook) validateIPPoolSubnet(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if err := spiderpoolip.IsCIDR(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet); err != nil {
		return field.Invalid(
			subnetField,
			ipPool.Spec.Subnet,
			err.Error(),
		)
	}

	var ipPoolList spiderpoolv1.SpiderIPPoolList
	if err := iw.Client.List(ctx, &ipPoolList); err != nil {
		return field.InternalError(subnetField, fmt.Errorf("failed to list IPPools: %v", err))
	}

	for _, pool := range ipPoolList.Items {
		if *pool.Spec.IPVersion == *ipPool.Spec.IPVersion {
			if pool.Name == ipPool.Name {
				return field.InternalError(subnetField, fmt.Errorf("IPPool %s already exists", ipPool.Name))
			}

			if pool.Spec.Subnet == ipPool.Spec.Subnet {
				continue
			}

			overlap, err := spiderpoolip.IsCIDROverlap(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, pool.Spec.Subnet)
			if err != nil {
				return field.InternalError(subnetField, fmt.Errorf("failed to compare whether 'spec.subnet' overlaps: %v", err))
			}

			if overlap {
				return field.Invalid(
					subnetField,
					ipPool.Spec.Subnet,
					fmt.Sprintf("overlap with IPPool %s which 'spec.subnet' is %s", pool.Name, pool.Spec.Subnet),
				)
			}
		}
	}

	return nil
}

func (iw *IPPoolWebhook) validateIPPoolAvailableIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	if err := iw.validateIPPoolIPs(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.IPs); err != nil {
		return err
	}
	if err := validateIPPoolExcludeIPs(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.ExcludeIPs); err != nil {
		return err
	}

	var ipPoolList spiderpoolv1.SpiderIPPoolList
	if err := iw.Client.List(ctx, &ipPoolList); err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to list IPPools: %v", err))
	}

	newIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %v", ipPool.Name, err))
	}

	for _, pool := range ipPoolList.Items {
		if pool.Spec.Subnet == ipPool.Spec.Subnet && pool.Name != ipPool.Name {
			existIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
			if err != nil {
				return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the existing IPPool %s: %v", pool.Name, err))
			}

			overlapIPs := spiderpoolip.IPsIntersectionSet(newIPs, existIPs)
			if len(overlapIPs) > 0 {
				overlapRanges, _ := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, overlapIPs)
				return field.Forbidden(
					ipsField,
					fmt.Sprintf("overlap with IPPool %s in IP ranges %v, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", pool.Name, overlapRanges),
				)
			}
		}
	}

	return nil
}

func (iw *IPPoolWebhook) validateIPPoolIPs(version types.IPVersion, subnet string, ips []string) *field.Error {
	if len(ips) == 0 {
		if iw.EnableSpiderSubnet {
			return nil
		}

		return field.Required(
			ipsField,
			"requires at least one item",
		)
	}

	for i, r := range ips {
		if err := ValidateContainsIPRange(ipsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func validateIPPoolExcludeIPs(version types.IPVersion, subnet string, excludeIPs []string) *field.Error {
	for i, r := range excludeIPs {
		if err := ValidateContainsIPRange(excludeIPsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func validateIPPoolGateway(version types.IPVersion, subnet string, gateway *string) *field.Error {
	if gateway != nil {
		return ValidateContainsIP(gatewayField, version, subnet, *gateway)
	}

	return nil
}

func validateIPPoolRoutes(version types.IPVersion, subnet string, routes []spiderpoolv1.Route) *field.Error {
	for i, r := range routes {
		if err := spiderpoolip.IsCIDR(version, r.Dst); err != nil {
			return field.Invalid(
				routesField.Index(i).Child("dst"),
				r.Dst,
				err.Error(),
			)
		}

		if err := ValidateContainsIP(routesField.Index(i).Child("gw"), version, subnet, r.Gw); err != nil {
			return err
		}
	}

	return nil
}

func ValidateContainsIPRange(fieldPath *field.Path, version types.IPVersion, subnet string, ipRange string) *field.Error {
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

func ValidateContainsIP(fieldPath *field.Path, version types.IPVersion, subnet string, ip string) *field.Error {
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
