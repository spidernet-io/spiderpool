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
	ipVersionField  *field.Path = field.NewPath("spec").Child("ipVersion")
	subnetField     *field.Path = field.NewPath("spec").Child("subnet")
	ipsField        *field.Path = field.NewPath("spec").Child("ips")
	excludeIPsField *field.Path = field.NewPath("spec").Child("excludeIPs")
	gatewayField    *field.Path = field.NewPath("spec").Child("gateway")
	routesField     *field.Path = field.NewPath("spec").Child("routes")
	freeIPsField    *field.Path = field.NewPath("status").Child("freeIPs")
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
	if err := sm.validateSubnetShouldNotBeChanged(ctx, oldSubnet, newSubnet); err != nil {
		return field.ErrorList{err}
	}

	if err := sm.validateSubnetSpec(ctx, newSubnet); err != nil {
		return field.ErrorList{err}
	}

	var errs field.ErrorList
	if err := sm.validateSubnetIPInUse(ctx, oldSubnet, newSubnet); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (sm *subnetManager) validateDeleteSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) field.ErrorList {
	var errs field.ErrorList
	if err := sm.validateSubnetIPPoolInUse(ctx, subnet); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func (sm *subnetManager) validateSubnetShouldNotBeChanged(ctx context.Context, oldSubnet, newSubnet *spiderpoolv1.SpiderSubnet) *field.Error {
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
	if err := sm.validateSubnetIPVersion(ctx, subnet.Spec.IPVersion); err != nil {
		return err
	}
	if err := sm.validateSubnetSubnet(ctx, *subnet.Spec.IPVersion, subnet.Name, subnet.Spec.Subnet); err != nil {
		return err
	}
	if err := sm.validateSubnetAvailableIP(ctx, subnet); err != nil {
		return err
	}
	if err := sm.validateSubnetGateway(ctx, *subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.Gateway); err != nil {
		return err
	}
	if err := sm.validateSubnetRoutes(ctx, *subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.Routes); err != nil {
		return err
	}

	return nil
}

func (sm *subnetManager) validateSubnetIPInUse(ctx context.Context, oldSubnet, newSubnet *spiderpoolv1.SpiderSubnet) *field.Error {
	if err := sm.validateSubnetIPs(ctx, *newSubnet.Spec.IPVersion, newSubnet.Spec.Subnet, newSubnet.Spec.IPs); err != nil {
		return err
	}
	if err := sm.validateSubnetExcludeIPs(ctx, *newSubnet.Spec.IPVersion, newSubnet.Spec.Subnet, newSubnet.Spec.ExcludeIPs); err != nil {
		return err
	}

	oldTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*oldSubnet.Spec.IPVersion, oldSubnet.Spec.IPs, oldSubnet.Spec.ExcludeIPs)
	newTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*newSubnet.Spec.IPVersion, newSubnet.Spec.IPs, newSubnet.Spec.ExcludeIPs)
	reducedIPs := spiderpoolip.IPsDiffSet(oldTotalIPs, newTotalIPs)
	freeIPs, err := spiderpoolip.ParseIPRanges(*newSubnet.Spec.IPVersion, newSubnet.Status.FreeIPs)
	if err != nil {
		return field.InternalError(freeIPsField, err)
	}

	invalidIPs := spiderpoolip.IPsDiffSet(reducedIPs, freeIPs)
	if len(invalidIPs) > 0 {
		ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*newSubnet.Spec.IPVersion, invalidIPs)
		return field.Forbidden(
			ipsField,
			fmt.Sprintf("remove some IP ranges %v that is being used, total IP addresses of an Subnet are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ranges),
		)
	}

	return nil
}

func (sm *subnetManager) validateSubnetIPPoolInUse(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) *field.Error {
	if err := sm.validateSubnetIPs(ctx, *subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.IPs); err != nil {
		return err
	}
	if err := sm.validateSubnetExcludeIPs(ctx, *subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.ExcludeIPs); err != nil {
		return err
	}

	totalIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	freeIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, subnet.Status.FreeIPs)
	if err != nil {
		return field.InternalError(freeIPsField, err)
	}

	if len(spiderpoolip.IPsDiffSet(totalIPs, freeIPs)) != 0 {
		return field.Forbidden(
			ipsField,
			"delete a Subnet that is still used by some IPPools",
		)
	}

	return nil
}

func (sm *subnetManager) validateSubnetIPVersion(ctx context.Context, version *types.IPVersion) *field.Error {
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

func (sm *subnetManager) validateSubnetAvailableIP(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) *field.Error {
	if err := sm.validateSubnetIPs(ctx, *subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.IPs); err != nil {
		return err
	}
	if err := sm.validateSubnetExcludeIPs(ctx, *subnet.Spec.IPVersion, subnet.Spec.Subnet, subnet.Spec.ExcludeIPs); err != nil {
		return err
	}

	subnetList, err := sm.ListSubnets(ctx)
	if err != nil {
		return field.InternalError(ipsField, err)
	}

	newIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	for _, s := range subnetList.Items {
		if s.Name == subnet.Name || s.Spec.Subnet != subnet.Spec.Subnet {
			continue
		}

		existIPs, err := spiderpoolip.AssembleTotalIPs(*s.Spec.IPVersion, s.Spec.IPs, s.Spec.ExcludeIPs)
		if err != nil {
			return field.InternalError(ipsField, err)
		}
		if len(newIPs) > len(spiderpoolip.IPsDiffSet(newIPs, existIPs)) {
			return field.Forbidden(
				ipsField,
				fmt.Sprintf("overlap with Subnet %s, total IP addresses of an Subnet are jointly determined by 'spec.ips' and 'spec.excludeIPs'", s.Name),
			)
		}
	}

	return nil
}

func (sm *subnetManager) validateSubnetIPs(ctx context.Context, version types.IPVersion, subnet string, ips []string) *field.Error {
	n := len(ips)
	if n == 0 {
		return field.Required(
			ipsField,
			"requires at least one item",
		)
	}

	for i, r := range ips {
		if err := ippoolmanager.ValidateContainsIPRange(ctx, ipsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func (sm *subnetManager) validateSubnetExcludeIPs(ctx context.Context, version types.IPVersion, subnet string, excludeIPs []string) *field.Error {
	for i, r := range excludeIPs {
		if err := ippoolmanager.ValidateContainsIPRange(ctx, excludeIPsField.Index(i), version, subnet, r); err != nil {
			return err
		}
	}

	return nil
}

func (sm *subnetManager) validateSubnetGateway(ctx context.Context, version types.IPVersion, subnet string, gateway *string) *field.Error {
	if gateway != nil {
		return ippoolmanager.ValidateContainsIP(ctx, gatewayField, version, subnet, *gateway)
	}

	return nil
}

func (sm *subnetManager) validateSubnetRoutes(ctx context.Context, version types.IPVersion, subnet string, routes []spiderpoolv1.Route) *field.Error {
	for i, r := range routes {
		if err := spiderpoolip.IsCIDR(version, r.Dst); err != nil {
			return field.Invalid(
				routesField.Index(i).Child("dst"),
				r.Dst,
				err.Error(),
			)
		}

		if err := ippoolmanager.ValidateContainsIP(ctx, routesField.Index(i).Child("gw"), version, subnet, r.Gw); err != nil {
			return err
		}
	}

	return nil
}
