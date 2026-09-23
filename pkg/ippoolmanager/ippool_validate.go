// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	apitypes "k8s.io/apimachinery/pkg/types"
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
	pairPoolField    *field.Path = field.NewPath("metadata").Child("annotations").Key(constant.AnnoIPPoolPairPool)
	nodeNameField    *field.Path = field.NewPath("spec").Child("nodeName")
	parentNicField   *field.Path = field.NewPath("metadata").Child("annotations").Key(constant.AnnoIPPoolParentNic)
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

	if err := iw.validatePairPool(ctx, ipPool); err != nil {
		errs = append(errs, err)
	}

	if err := validateIaasParentNic(ipPool); err != nil {
		errs = append(errs, err)
	}

	if err := validateIaasSingleNode(ipPool); err != nil {
		errs = append(errs, err)
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

	if err := iw.validatePairPool(ctx, newIPPool); err != nil {
		errs = append(errs, err)
	}

	if err := validateIaasNodeNameImmutable(oldIPPool, newIPPool); err != nil {
		errs = append(errs, err)
	}

	if err := validateIaasParentNic(newIPPool); err != nil {
		errs = append(errs, err)
	}

	if err := validateIaasSingleNode(newIPPool); err != nil {
		errs = append(errs, err)
	}

	if errorList := validateIaasAnnotationsImmutableWithAllocatedIPs(oldIPPool, newIPPool); len(errorList) != 0 {
		errs = append(errs, errorList...)
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

// validatePairPool enforces the pairing rules for the
// ipam.spidernet.io/pair-pool annotation (data-model.md §2):
//   - no self-reference
//   - the referenced pool, when it exists, must be of the opposite IP version
//   - the v4 pool's static capacity (spec.ips minus spec.excludeIPs) must be
//     <= the v6 pool's static capacity
//   - the two pools' spec.nodeName and spec.podAffinity must be identical
//
// A reference to a not-yet-existing pool is explicitly allowed; convergence
// happens once the second pool is created.
func (iw *IPPoolWebhook) validatePairPool(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	pairName, ok := ipPool.Annotations[constant.AnnoIPPoolPairPool]
	if !ok || pairName == "" {
		return nil
	}

	if pairName == ipPool.Name {
		return field.Invalid(pairPoolField, pairName, "cannot reference itself as a pair pool")
	}

	var pairPool spiderpoolv2beta1.SpiderIPPool
	if err := iw.APIReader.Get(ctx, apitypes.NamespacedName{Name: pairName}, &pairPool); err != nil {
		if apierrors.IsNotFound(err) {
			// The paired pool may not exist yet; convergence happens later.
			return nil
		}
		return field.InternalError(pairPoolField, fmt.Errorf("failed to get pair IPPool %s: %w", pairName, err))
	}

	if ipPool.Spec.IPVersion != nil && pairPool.Spec.IPVersion != nil &&
		*ipPool.Spec.IPVersion == *pairPool.Spec.IPVersion {
		return field.Invalid(pairPoolField, pairName, "must reference a pool of the opposite IP version")
	}

	v4Pool, v6Pool := ipPool, &pairPool
	if ipPool.Spec.IPVersion != nil && *ipPool.Spec.IPVersion == constant.IPv6 {
		v4Pool, v6Pool = &pairPool, ipPool
	}

	if v4Pool.Spec.IPVersion != nil && v6Pool.Spec.IPVersion != nil {
		v4Capacity, err := poolStaticCapacity(v4Pool)
		if err != nil {
			return field.InternalError(pairPoolField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %w", v4Pool.Name, err))
		}
		v6Capacity, err := poolStaticCapacity(v6Pool)
		if err != nil {
			return field.InternalError(pairPoolField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %w", v6Pool.Name, err))
		}

		if v4Capacity > v6Capacity {
			return field.Forbidden(pairPoolField, fmt.Sprintf("v4 pool %s static capacity (%d) must be <= v6 pool %s static capacity (%d)", v4Pool.Name, v4Capacity, v6Pool.Name, v6Capacity))
		}
	}

	if !reflect.DeepEqual(ipPool.Spec.NodeName, pairPool.Spec.NodeName) {
		return field.Forbidden(pairPoolField, fmt.Sprintf("'spec.nodeName' must match pair IPPool %s's 'spec.nodeName'", pairName))
	}

	if !reflect.DeepEqual(ipPool.Spec.PodAffinity, pairPool.Spec.PodAffinity) {
		return field.Forbidden(pairPoolField, fmt.Sprintf("'spec.podAffinity' must match pair IPPool %s's 'spec.podAffinity'", pairName))
	}

	return nil
}

// validateIaasNodeNameImmutable keeps an IaaS pool's mode (node-level
// prewarm vs. global) stable for its lifetime. The mode is derived solely
// from whether spec.nodeName is set (set → node-level prewarm pool; empty →
// global pool, see IsGlobalIaaSPool), so adding nodeName to a global pool or
// removing it from a node-level pool would silently flip the allocation path
// and the external provider's behavior mid-flight. Changing between two
// non-empty node lists is not a mode flip and stays allowed; non-IaaS pools
// are unaffected.
func validateIaasNodeNameImmutable(oldIPPool, newIPPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	if _, ok := oldIPPool.Annotations[constant.AnnoIPPoolIaasProvider]; !ok {
		return nil
	}

	if (len(oldIPPool.Spec.NodeName) == 0) != (len(newIPPool.Spec.NodeName) == 0) {
		return field.Forbidden(
			nodeNameField,
			"cannot add or remove 'spec.nodeName' on an IaaS pool: the pool mode (node-level prewarm vs. global) is derived from it and is immutable",
		)
	}

	return nil
}

// validateIaasParentNic enforces the rules for the
// ipam.spidernet.io/parent-nic annotation, the single guest-OS parent NIC
// name the external IaaS provider exchanges for a MAC through the node
// annotation ipam.spidernet.io/parent-nics. A node-scoped IaaS pool
// (iaas-provider annotation plus a non-empty spec.nodeName) prewarms
// exclusively through this annotation, so it is required there and its
// absence fails fast at admission instead of surfacing as a prewarm
// failure. Whenever present (node-scoped or global pool alike), the value
// must be a single NIC name: non-blank after trimming and free of commas
// and embedded whitespace.
func validateIaasParentNic(ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	parentNic, ok := ipPool.Annotations[constant.AnnoIPPoolParentNic]

	if !ok {
		_, isIaasProvider := ipPool.Annotations[constant.AnnoIPPoolIaasProvider]
		if isIaasProvider && len(ipPool.Spec.NodeName) != 0 {
			return field.Required(
				parentNicField,
				fmt.Sprintf("node-scoped IaaS pool requires annotation %s (a single parent NIC name)", constant.AnnoIPPoolParentNic),
			)
		}
		return nil
	}

	trimmed := strings.TrimSpace(parentNic)
	if trimmed == "" || strings.Contains(trimmed, ",") || strings.ContainsFunc(trimmed, unicode.IsSpace) {
		return field.Invalid(
			parentNicField,
			parentNic,
			fmt.Sprintf("%s must be a single NIC name", constant.AnnoIPPoolParentNic),
		)
	}

	return nil
}

// validateIaasSingleNode pins an IaaS node-level (prewarm) pool to exactly
// one node: the agent on that node publishes the single parent NIC MAC to
// status.parentNic, which cannot represent per-node MACs of a multi-node
// pool. Non-IaaS pools and IaaS global pools (empty spec.nodeName) are
// unaffected.
func validateIaasSingleNode(ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	if _, ok := ipPool.Annotations[constant.AnnoIPPoolIaasProvider]; !ok {
		return nil
	}
	if len(ipPool.Spec.NodeName) > 1 {
		return field.Forbidden(
			nodeNameField,
			"an IaaS node-level pool must be pinned to exactly one node: its status.parentNic carries the single parent NIC MAC of that node",
		)
	}
	return nil
}

// validateIaasAnnotationsImmutableWithAllocatedIPs forbids removing or
// modifying the IaaS marker annotations (iaas-provider and parent-nic)
// on a pool that still has allocated IPs: those markers decide
// whether and how the external provider is involved in the release path, so
// flipping them mid-flight would strand cloud-side sub-ENI state. Adding a
// previously absent annotation stays allowed, as it cannot invalidate
// existing allocations.
func validateIaasAnnotationsImmutableWithAllocatedIPs(oldIPPool, newIPPool *spiderpoolv2beta1.SpiderIPPool) field.ErrorList {
	if oldIPPool.Status.AllocatedIPCount == nil || *oldIPPool.Status.AllocatedIPCount <= 0 {
		return nil
	}

	var errs field.ErrorList
	for _, key := range []string{constant.AnnoIPPoolIaasProvider, constant.AnnoIPPoolParentNic} {
		oldVal, oldOk := oldIPPool.Annotations[key]
		if !oldOk {
			continue
		}
		annoField := field.NewPath("metadata").Child("annotations").Key(key)
		newVal, newOk := newIPPool.Annotations[key]
		if !newOk {
			errs = append(errs, field.Forbidden(
				annoField,
				fmt.Sprintf("cannot remove annotation %s while the IPPool has allocated IPs", key),
			))
			continue
		}
		if newVal != oldVal {
			errs = append(errs, field.Forbidden(
				annoField,
				fmt.Sprintf("cannot modify annotation %s while the IPPool has allocated IPs", key),
			))
		}
	}

	return errs
}

// poolStaticCapacity returns the number of usable static addresses of a pool
// (spec.ips minus spec.excludeIPs).
func poolStaticCapacity(ipPool *spiderpoolv2beta1.SpiderIPPool) (int, error) {
	totalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return 0, err
	}
	return len(totalIPs), nil
}
