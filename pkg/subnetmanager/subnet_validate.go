// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	defaultField           *field.Path = field.NewPath("spec").Child("default")
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
	if err := sw.validateSubnetDefault(ctx, subnet); err != nil {
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

	// TODO(iiiceoo): Use label selector.
	subnetList := spiderpoolv2beta1.SpiderSubnetList{}
	if err := sw.APIReader.List(ctx, &subnetList); err != nil {
		return field.InternalError(subnetField, fmt.Errorf("failed to list Subnets: %v", err))
	}

	for _, s := range subnetList.Items {
		if *s.Spec.IPVersion == *subnet.Spec.IPVersion {
			if s.Name == subnet.Name {
				return field.InternalError(subnetField, fmt.Errorf("subnet %s already exists", subnet.Name))
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

	return nil
}

func (sw *SubnetWebhook) validateSubnetDefault(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	if subnet.Spec.Default == nil || !*subnet.Spec.Default {
		return nil
	}

	var subnetList spiderpoolv2beta1.SpiderSubnetList
	if err := sw.Client.List(
		ctx,
		&subnetList,
		client.MatchingFields{"spec.default": strconv.FormatBool(true)},
	); err != nil {
		return field.InternalError(defaultField, fmt.Errorf("failed to list default Subnets: %v", err))
	}

	for _, ds := range subnetList.Items {
		if *ds.Spec.IPVersion == *subnet.Spec.IPVersion {
			if ds.Name != subnet.Name {
				return field.Forbidden(
					defaultField,
					fmt.Sprintf("Subnet %s has been set as the default Subnet, and there is only one default Subnet in the cluster", ds.Name),
				)
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

func validateSubnetGateway(version types.IPVersion, subnet string, gateway *string) *field.Error {
	if gateway != nil {
		return ippoolmanager.ValidateContainsIP(gatewayField, version, subnet, *gateway)
	}

	return nil
}

func validateSubnetRoutes(version types.IPVersion, subnet string, routes []spiderpoolv2beta1.Route) *field.Error {
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
