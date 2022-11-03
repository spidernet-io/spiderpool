// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
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

func (sm *subnetManager) validateCreateSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) field.ErrorList {
	var errs field.ErrorList
	if err := sm.validateSubnetSpec(ctx, subnet); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (sm *subnetManager) validateUpdateSubnet(ctx context.Context, oldSubnet, newSubnet *spiderpoolv1.SpiderSubnet) field.ErrorList {
	if err := validateSubnetShouldNotBeChanged(oldSubnet, newSubnet); err != nil {
		return field.ErrorList{err}
	}

	if err := sm.validateSubnetSpec(ctx, newSubnet); err != nil {
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

func validateSubnetShouldNotBeChanged(oldSubnet, newSubnet *spiderpoolv1.SpiderSubnet) *field.Error {
	if *newSubnet.Spec.IPVersion != *oldSubnet.Spec.IPVersion {
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

func (sm *subnetManager) validateSubnetSpec(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) *field.Error {
	if err := sm.validateSubnetIPVersion(subnet.Spec.IPVersion); err != nil {
		return err
	}
	if err := sm.validateSubnetSubnet(ctx, *subnet.Spec.IPVersion, subnet.Name, subnet.Spec.Subnet); err != nil {
		return err
	}
	if err := validateSubnetIPs(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.IPs); err != nil {
		return err
	}
	if err := validateSubnetExcludeIPs(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.ExcludeIPs); err != nil {
		return err
	}
	if err := validateSubnetGateway(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.Gateway); err != nil {
		return err
	}

	return validateSubnetRoutes(*subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.Routes)
}

func validateSubnetIPInUse(subnet *spiderpoolv1.SpiderSubnet) *field.Error {
	totalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, err)
	}

	for poolName, preAllocation := range subnet.Status.ControlledIPPools {
		poolTotalIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, preAllocation.IPs)
		if err != nil {
			return field.InternalError(controlledIPPoolsField, err)
		}
		invalidIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, totalIPs)
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

func (sm *subnetManager) validateSubnetIPVersion(version *types.IPVersion) *field.Error {
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

	if *version == constant.IPv4 && !sm.config.EnableIPv4 {
		return field.Forbidden(
			ipVersionField,
			"IPv4 is disabled",
		)
	}

	if *version == constant.IPv6 && !sm.config.EnableIPv6 {
		return field.Forbidden(
			ipVersionField,
			"IPv6 is disabled",
		)
	}

	return nil
}

func (sm *subnetManager) validateSubnetSubnet(ctx context.Context, version types.IPVersion, subnetName, subnet string) *field.Error {
	subnetList, err := sm.ListSubnets(ctx)
	if err != nil {
		return field.InternalError(subnetField, err)
	}

	for _, s := range subnetList.Items {
		if s.Name == subnetName || *s.Spec.IPVersion != version {
			continue
		}

		overlap, err := spiderpoolip.IsCIDROverlap(version, subnet, s.Spec.Subnet)
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
				fmt.Sprintf("overlap with Subnet %s which 'spec.subnet' is %s", s.Name, s.Spec.Subnet),
			)
		}
	}

	return nil
}

func validateSubnetIPs(version types.IPVersion, subnet string, ips []string) *field.Error {
	n := len(ips)
	if n == 0 {
		return field.Required(
			ipsField,
			"requires at least one item",
		)
	}

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

func validateSubnetGateway(version types.IPVersion, subnet string, gateway *string) *field.Error {
	if gateway != nil {
		return ippoolmanager.ValidateContainsIP(gatewayField, version, subnet, *gateway)
	}

	return nil
}

func validateSubnetRoutes(version types.IPVersion, subnet string, routes []spiderpoolv1.Route) *field.Error {
	for i, r := range routes {
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
