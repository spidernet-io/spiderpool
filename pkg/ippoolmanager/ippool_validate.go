// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

var (
	ipVersionField   *field.Path = field.NewPath("spec").Child("ipVersion")
	subnetField      *field.Path = field.NewPath("spec").Child("subnet")
	ipsField         *field.Path = field.NewPath("spec").Child("ips")
	gatewayField     *field.Path = field.NewPath("spec").Child("gateway")
	routesField      *field.Path = field.NewPath("spec").Child("routes")
	podAffinityField *field.Path = field.NewPath("spec").Child("podAffinity")
)

func (iw *IPPoolWebhook) validateCreateIPPool(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) field.ErrorList {
	if err := iw.validateIPPoolIPVersion(ipPool.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	if err := iw.validateIPPoolCIDR(ctx, ipPool); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := iw.validateIPPoolSpec(ctx, ipPool); err != nil {
		errs = append(errs, err)
	}

	errorList := validateIPPoolPodAffinity(podAffinityField, ipPool)
	if len(errorList) != 0 {
		errs = append(errs, errorList...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (iw *IPPoolWebhook) validateUpdateIPPool(ctx context.Context, oldIPPool, newIPPool *spiderpoolv2beta1.SpiderIPPool) field.ErrorList {
	if err := validateIPPoolShouldNotBeChanged(oldIPPool, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	if err := iw.validateIPPoolIPVersion(newIPPool.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	if err := iw.validateIPPoolSpec(ctx, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	errorList := validateIPPoolPodAffinity(podAffinityField, newIPPool)
	if len(errorList) != 0 {
		return errorList
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

func validateIPPoolShouldNotBeChanged(oldIPPool, newIPPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
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

func (iw *IPPoolWebhook) validateIPPoolSpec(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	if err := iw.validateIPPoolAvailableIPs(ctx, ipPool); err != nil {
		return err
	}
	if err := validateIPPoolGateway(ipPool); err != nil {
		return err
	}

	return validateIPPoolRoutes(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, ipPool.Spec.Routes)
}

func validateIPPoolIPInUse(ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to unmarshal the allocated IP records of IPPool %s: %w", ipPool.Name, err))
	}

	totalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %w", ipPool.Name, err))
	}

	totalIPsMap := map[string]bool{}
	for _, ip := range totalIPs {
		totalIPsMap[ip.String()] = true
	}

	for ip, allocation := range allocatedRecords {
		if _, ok := totalIPsMap[ip]; !ok {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("remove an IP address %s that is being used by Pod %s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ip, allocation.NamespacedName),
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
			[]string{
				strconv.FormatInt(constant.IPv4, 10),
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

func (iw *IPPoolWebhook) validateIPPoolCIDR(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	if err := spiderpoolip.IsCIDR(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet); err != nil {
		return field.Invalid(
			subnetField,
			ipPool.Spec.Subnet,
			err.Error(),
		)
	}
	if err := spiderpoolip.IsFormatCIDR(ipPool.Spec.Subnet); err != nil {
		return field.Invalid(subnetField, ipPool.Spec.Subnet, err.Error())
	}

	var ipPoolList spiderpoolv2beta1.SpiderIPPoolList
	if err := iw.APIReader.List(ctx, &ipPoolList); err != nil {
		return field.InternalError(subnetField, fmt.Errorf("failed to list IPPools: %w", err))
	}

	for _, pool := range ipPoolList.Items {
		if *pool.Spec.IPVersion == *ipPool.Spec.IPVersion {
			// since we met already exist IPPool resource, we just return the error to avoid the following taxing operations.
			// the user can also use k8s 'errors.IsAlreadyExists' to get the right error type assertion.
			if pool.Name == ipPool.Name {
				return field.InternalError(subnetField, fmt.Errorf("IPPool %s %s", ipPool.Name, metav1.StatusReasonAlreadyExists))
			}

			if pool.Spec.Subnet == ipPool.Spec.Subnet {
				continue
			}

			overlap, err := spiderpoolip.IsCIDROverlap(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, pool.Spec.Subnet)
			if err != nil {
				return field.InternalError(subnetField, fmt.Errorf("failed to compare whether 'spec.subnet' overlaps: %w", err))
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

func (iw *IPPoolWebhook) validateIPPoolAvailableIPs(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	newPool, err := spiderpoolip.NewCIDR(ipPool.Spec.Subnet, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.Invalid(subnetField, ipPool.Spec.Subnet, err.Error())
	}

	cidr, err := spiderpoolip.CIDRToLabelValue(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to parse CIDR %s as a valid label value: %w", ipPool.Spec.Subnet, err))
	}

	var ipPoolList spiderpoolv2beta1.SpiderIPPoolList
	if err := iw.APIReader.List(
		ctx,
		&ipPoolList,
		client.MatchingLabels{constant.LabelIPPoolCIDR: cidr},
	); err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to list IPPools: %w", err))
	}

	for _, pool := range ipPoolList.Items {
		if pool.Name != ipPool.Name {
			existPool, err := spiderpoolip.NewCIDR(pool.Spec.Subnet, pool.Spec.IPs, pool.Spec.ExcludeIPs)
			if err != nil {
				return field.Invalid(subnetField, pool.Spec.Subnet, err.Error())
			}
			if overlapRanges, isOverlap := newPool.IsOverlapIPRanges(existPool.IPRange()); isOverlap {
				return field.Forbidden(
					ipsField,
					fmt.Sprintf("overlap with IPPool %s in IP ranges %v, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", pool.Name, overlapRanges),
				)
			}
		}
	}

	return nil
}

func validateIPPoolGateway(ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	if ipPool.Spec.Gateway == nil {
		return nil
	}

	if err := ValidateContainsIP(gatewayField, *ipPool.Spec.IPVersion, ipPool.Spec.Subnet, *ipPool.Spec.Gateway); err != nil {
		return err
	}

	for _, r := range ipPool.Spec.ExcludeIPs {
		contains, _ := spiderpoolip.IPRangeContainsIP(*ipPool.Spec.IPVersion, r, *ipPool.Spec.Gateway)
		if contains {
			return nil
		}
	}

	for i, r := range ipPool.Spec.IPs {
		contains, _ := spiderpoolip.IPRangeContainsIP(*ipPool.Spec.IPVersion, r, *ipPool.Spec.Gateway)
		if contains {
			return field.Invalid(
				ipsField.Index(i),
				r,
				fmt.Sprintf("conflicts with 'spec.gateway' %s, add the gateway IP address to 'spec.excludeIPs' or remove it from 'spec.ips'", *ipPool.Spec.Gateway),
			)
		}
	}

	return nil
}

func validateIPPoolRoutes(version types.IPVersion, subnet string, routes []spiderpoolv2beta1.Route) *field.Error {
	if len(routes) == 0 {
		return nil
	}

	dstSet := make(map[string]bool, len(routes))
	for i, r := range routes {
		if version == constant.IPv4 && r.Dst == "0.0.0.0/0" ||
			version == constant.IPv6 && r.Dst == "::/0" {
			return field.Invalid(
				routesField.Index(i).Child("dst"),
				r.Dst,
				"please specify 'spec.gateway' to configure the default route",
			)
		}

		if _, ok := dstSet[r.Dst]; ok {
			return field.Invalid(
				routesField.Index(i).Child("dst"),
				r.Dst,
				"duplicate route with the same dst",
			)
		}
		dstSet[r.Dst] = true

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

func validateIPPoolPodAffinity(fieldPath *field.Path, ipPool *spiderpoolv2beta1.SpiderIPPool) field.ErrorList {
	if ipPool.Spec.PodAffinity == nil {
		return nil
	}

	var allErrs field.ErrorList
	// auto-created IPPool special podAffinity validation
	if IsAutoCreatedIPPool(ipPool) {
		for k := range ipPool.Spec.PodAffinity.MatchLabels {
			if !slices.Contains(constant.AutoPoolPodAffinities, k) {
				allErrs = append(allErrs, field.Invalid(podAffinityField, ipPool.Spec.PodAffinity,
					"it's invalid to add additional podAffinity matchLabels for auto-created SpiderIPPool"))
			}
		}

		if len(ipPool.Spec.PodAffinity.MatchExpressions) != 0 {
			allErrs = append(allErrs, field.Invalid(podAffinityField, ipPool.Spec.PodAffinity,
				"it's invalid to add additional podAffinity matchExpressions for auto-created SpiderIPPool"))
		}

		return allErrs
	}

	// normal IPPool podAffinity validation
	errList := validation.ValidateLabelSelector(ipPool.Spec.PodAffinity,
		validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: false},
		fieldPath)
	if errList != nil {
		allErrs = append(allErrs, errList...)
	}

	if len(ipPool.Spec.PodAffinity.MatchLabels)+len(ipPool.Spec.PodAffinity.MatchExpressions) == 0 {
		allErrs = append(allErrs, field.Invalid(podAffinityField, ipPool.Spec.PodAffinity, "empty podAffinity is invalid for SpiderIPPool"))
	}

	return allErrs
}
