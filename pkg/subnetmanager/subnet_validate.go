// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

var (
	ipVersionField         *field.Path = field.NewPath("spec").Child("ipVersion")
	subnetField            *field.Path = field.NewPath("spec").Child("subnet")
	ipsField               *field.Path = field.NewPath("spec").Child("ips")
	excludeIPsField        *field.Path = field.NewPath("spec").Child("excludeIPs")
	gatewayField           *field.Path = field.NewPath("spec").Child("gateway")
	routesField            *field.Path = field.NewPath("spec").Child("routes")
	controlledIPPoolsField *field.Path = field.NewPath("status").Child("controlledIPPools")
)

func (sw *SubnetWebhook) validateCreateSubnet(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) field.ErrorList {
	if err := sw.validateSubnetIPVersion(subnet.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	if err := sw.validateSubnetCIDR(ctx, subnet); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := sw.validateSubnetSpec(ctx, subnet); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (sw *SubnetWebhook) validateUpdateSubnet(ctx context.Context, oldSubnet, newSubnet *spiderpoolv2beta1.SpiderSubnet) field.ErrorList {
	if err := validateSubnetShouldNotBeChanged(oldSubnet, newSubnet); err != nil {
		return field.ErrorList{err}
	}

	if err := sw.validateSubnetIPVersion(newSubnet.Spec.IPVersion); err != nil {
		return field.ErrorList{err}
	}

	if err := sw.validateSubnetSpec(ctx, newSubnet); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := validateSubnetIPInUse(newSubnet); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func validateSubnetShouldNotBeChanged(oldSubnet, newSubnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	if newSubnet.Spec.IPVersion != nil && oldSubnet.Spec.IPVersion != nil &&
		*newSubnet.Spec.IPVersion != *oldSubnet.Spec.IPVersion {
		return field.Forbidden(
			ipVersionField,
			"is not changeable",
		)
	}

	if newSubnet.Spec.Subnet != oldSubnet.Spec.Subnet {
		return field.Forbidden(
			subnetField,
			"is not changeable",
		)
	}

	return nil
}

func (sw *SubnetWebhook) validateSubnetSpec(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	if err := validateSubnetIPs(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.IPs); err != nil {
		return err
	}
	if err := validateSubnetExcludeIPs(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.ExcludeIPs); err != nil {
		return err
	}
	if err := validateSubnetGateway(subnet); err != nil {
		return err
	}

	return validateSubnetRoutes(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.Routes)
}

func validateSubnetIPInUse(subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	totalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the Subnet %s: %v", subnet.Name, err))
	}

	if subnet.Status.ControlledIPPools == nil {
		return nil
	}

	preAllocations, err := convert.UnmarshalSubnetAllocatedIPPools(subnet.Status.ControlledIPPools)
	if err != nil {
		return field.InternalError(controlledIPPoolsField, fmt.Errorf("failed to unmarshal the controlled IPPools of Subnet %s: %v", subnet.Name, err))
	}

	for poolName, preAllocation := range preAllocations {
		poolTotalIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, preAllocation.IPs)
		if err != nil {
			return field.InternalError(controlledIPPoolsField, fmt.Errorf("failed to parse the pre-allocation of the IPPool %s: %v", poolName, err))
		}
		invalidIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, totalIPs, false)
		if len(invalidIPs) > 0 {
			ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*subnet.Spec.IPVersion, invalidIPs)
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("remove some IP ranges %v that is being used by IPPool %s, total IP addresses of an Subnet are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ranges, poolName),
			)
		}
	}

	return nil
}

func (sw *SubnetWebhook) validateSubnetIPVersion(version *types.IPVersion) *field.Error {
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

	if *version == constant.IPv4 && !sw.EnableIPv4 {
		return field.Forbidden(
			ipVersionField,
			"IPv4 is disabled",
		)
	}

	if *version == constant.IPv6 && !sw.EnableIPv6 {
		return field.Forbidden(
			ipVersionField,
			"IPv6 is disabled",
		)
	}

	return nil
}

func (sw *SubnetWebhook) validateSubnetCIDR(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	if err := spiderpoolip.IsCIDR(*subnet.Spec.IPVersion, subnet.Spec.Subnet); err != nil {
		return field.Invalid(
			subnetField,
			subnet.Spec.Subnet,
			err.Error(),
		)
	}
	if err := spiderpoolip.IsFormatCIDR(subnet.Spec.Subnet); err != nil {
		return field.Invalid(subnetField, subnet.Spec.Subnet, err.Error())
	}

	subnetList := spiderpoolv2beta1.SpiderSubnetList{}
	if err := sw.APIReader.List(ctx, &subnetList); err != nil {
		return field.InternalError(subnetField, fmt.Errorf("failed to list Subnets: %v", err))
	}

	for _, s := range subnetList.Items {
		if *s.Spec.IPVersion == *subnet.Spec.IPVersion {
			// since we met already exist Subnet resource, we just return the error to avoid the following taxing operations.
			// the user can also use k8s 'errors.IsAlreadyExists' to get the right error type assertion.
			if s.Name == subnet.Name {
				return field.InternalError(subnetField, fmt.Errorf("subnet %s %s", subnet.Name, metav1.StatusReasonAlreadyExists))
			}

			overlap, err := spiderpoolip.IsCIDROverlap(*subnet.Spec.IPVersion, subnet.Spec.Subnet, s.Spec.Subnet)
			if err != nil {
				return field.InternalError(subnetField, fmt.Errorf("failed to compare whether 'spec.subnet' overlaps: %v", err))
			}

			if overlap {
				return field.Invalid(
					subnetField,
					subnet.Spec.Subnet,
					fmt.Sprintf("overlap with Subnet %s which 'spec.subnet' is %s", s.Name, s.Spec.Subnet),
				)
			}
		}
	}

	return sw.validateOrphanIPPool(ctx, subnet)
}

// validateOrphanIPPool will check the SpiderSubnet.Spec.Subnet whether overlaps with the cluster orphan SpiderIPPool.Spec.Subnet.
// And we also require the IPPool.Spec.IPs belong to Subnet.Spec.IPs if they are in the same subnet
func (sw *SubnetWebhook) validateOrphanIPPool(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	poolList := spiderpoolv2beta1.SpiderIPPoolList{}
	err := sw.APIReader.List(ctx, &poolList)
	if nil != err {
		return field.InternalError(subnetField, fmt.Errorf("failed to list IPPools: %v", err))
	}

	for _, tmpPool := range poolList.Items {
		if *tmpPool.Spec.IPVersion != *subnet.Spec.IPVersion {
			continue
		}

		// validate the Spec.Subnet whether overlaps or not
		if tmpPool.Spec.Subnet != subnet.Spec.Subnet {
			isCIDROverlap, err := spiderpoolip.IsCIDROverlap(*subnet.Spec.IPVersion, tmpPool.Spec.Subnet, subnet.Spec.Subnet)
			if nil != err {
				return field.InternalError(subnetField, fmt.Errorf("failed to compare whether 'spec.subnet' overlaps with SpiderIPPool '%s', error: %v", tmpPool.Name, err))
			}
			if isCIDROverlap {
				return field.Invalid(subnetField, subnet.Spec.Subnet, fmt.Sprintf("overlap with SpiderIPPool '%s' resource 'spec.subnet' %s", tmpPool.Name, tmpPool.Spec.Subnet))
			}
		} else {
			// validate the Spec.IPs whether contains or not
			poolIPs, err := spiderpoolip.AssembleTotalIPs(*tmpPool.Spec.IPVersion, tmpPool.Spec.IPs, tmpPool.Spec.ExcludeIPs)
			if nil != err {
				return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool '%s', error: %v", tmpPool.Name, err))
			}
			subnetIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
			if nil != err {
				return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the Subnet '%s', error: %v", subnet.Name, err))
			}
			if spiderpoolip.IsDiffIPSet(poolIPs, subnetIPs) {
				return field.Invalid(ipsField, subnet.Spec.IPs, fmt.Sprintf("SpiderIPPool '%s' owns some IP addresses that SpiderSubnet '%s' can't control", tmpPool.Name, subnet.Name))
			}
		}
	}

	return nil
}

func validateSubnetIPs(version types.IPVersion, subnet string, ips []string) *field.Error {
	for i, r := range ips {
		if err := ippoolmanager.ValidateContainsIPRange(ipsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func validateSubnetExcludeIPs(version types.IPVersion, subnet string, excludeIPs []string) *field.Error {
	for i, r := range excludeIPs {
		if err := ippoolmanager.ValidateContainsIPRange(excludeIPsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func validateSubnetGateway(subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	if subnet.Spec.Gateway == nil {
		return nil
	}

	if err := ippoolmanager.ValidateContainsIP(gatewayField, *subnet.Spec.IPVersion, subnet.Spec.Subnet, *subnet.Spec.Gateway); err != nil {
		return err
	}

	for _, r := range subnet.Spec.ExcludeIPs {
		contains, _ := spiderpoolip.IPRangeContainsIP(*subnet.Spec.IPVersion, r, *subnet.Spec.Gateway)
		if contains {
			return nil
		}
	}

	for i, r := range subnet.Spec.IPs {
		contains, _ := spiderpoolip.IPRangeContainsIP(*subnet.Spec.IPVersion, r, *subnet.Spec.Gateway)
		if contains {
			return field.Invalid(
				ipsField.Index(i),
				r,
				fmt.Sprintf("conflicts with 'spec.gateway' %s, add the gateway IP address to 'spec.excludeIPs' or remove it from 'spec.ips'", *subnet.Spec.Gateway),
			)
		}
	}

	return nil
}

func validateSubnetRoutes(version types.IPVersion, subnet string, routes []spiderpoolv2beta1.Route) *field.Error {
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

		if err := ippoolmanager.ValidateContainsIP(routesField.Index(i).Child("gw"), version, subnet, r.Gw); err != nil {
			return err
		}
	}

	return nil
}
