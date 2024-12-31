// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

func (iw *IPPoolWebhook) validateCreateIPPoolWhileEnableSpiderSubnet(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) field.ErrorList {
	if errs := iw.validateCreateIPPool(ctx, ipPool); len(errs) != 0 {
		return errs
	}

	if iw.EnableSpiderSubnet {
		if err := iw.validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx, ipPool); err != nil {
			return field.ErrorList{err}
		}
	}

	return nil
}

func (iw *IPPoolWebhook) validateUpdateIPPoolWhileEnableSpiderSubnet(ctx context.Context, oldIPPool, newIPPool *spiderpoolv2beta1.SpiderIPPool) field.ErrorList {
	if errs := iw.validateUpdateIPPool(ctx, oldIPPool, newIPPool); len(errs) != 0 {
		return errs
	}

	if iw.EnableSpiderSubnet {
		if err := iw.validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx, newIPPool); err != nil {
			return field.ErrorList{err}
		}
	}

	return nil
}

func (iw *IPPoolWebhook) validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) *field.Error {
	owner := metav1.GetControllerOf(ipPool)
	if owner == nil {
		return nil
	}

	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %v", ipPool.Name, err))
	}
	if len(poolTotalIPs) == 0 {
		return nil
	}

	var subnet spiderpoolv2beta1.SpiderSubnet
	if err := iw.APIReader.Get(ctx, apitypes.NamespacedName{Name: owner.Name}, &subnet); err != nil {
		return field.InternalError(subnetField, fmt.Errorf("failed to get controller Subnet %s: %v", owner.Name, err))
	}

	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the Subnet %s: %v", subnet.Name, err))
	}

	outIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, subnetTotalIPs, false)
	if len(outIPs) > 0 {
		ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*ipPool.Spec.IPVersion, outIPs)
		return field.Forbidden(
			ipsField,
			fmt.Sprintf("add some IP ranges %v that are not contained in controller Subnet %s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ranges, subnet.Name),
		)
	}

	if IsAutoCreatedIPPool(ipPool) {
		return validateNewAutoPoolTotalIPsWithinSubnet(ipPool, &subnet)
	}

	return nil
}

func validateNewAutoPoolTotalIPsWithinSubnet(pool *spiderpoolv2beta1.SpiderIPPool, subnet *spiderpoolv2beta1.SpiderSubnet) *field.Error {
	var subnetPreAllocateIPs []net.IP

	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the Subnet %s: %v", subnet.Name, err))
	}
	sort.Slice(poolTotalIPs, func(i, j int) bool {
		return bytes.Compare(poolTotalIPs[i].To16(), poolTotalIPs[j].To16()) < 0
	})

	subnetAllocatedIPPools, err := convert.UnmarshalSubnetAllocatedIPPools(subnet.Status.ControlledIPPools)
	if nil != err {
		return field.InternalError(subnetField, fmt.Errorf("failed unsharmal SpiderSubnet %s Status.ControlledIPPools: %v", subnet.Name, err))
	}
	if subnetAllocatedIPPools != nil {
		subnetPoolAllocation, ok := subnetAllocatedIPPools[pool.Name]
		if ok {
			subnetPoolIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, subnetPoolAllocation.IPs)
			if nil != err {
				return field.InternalError(subnetField, fmt.Errorf("failed to parse SpiderSubnet %s controlledIPPool %s IPs: %v ", subnet.Name, pool.Name, err))
			}
			sort.Slice(subnetPoolIPs, func(i, j int) bool {
				return bytes.Compare(subnetPoolIPs[i].To16(), subnetPoolIPs[j].To16()) < 0
			})
			subnetPreAllocateIPs = subnetPoolIPs
		}
	}

	isEqual := reflect.DeepEqual(subnetPreAllocateIPs, poolTotalIPs)
	if !isEqual {
		return field.Forbidden(ipsField,
			"it's illegal to update AutoPool.Spec.IPs that are different from corresponding SpiderSubnet.Status.ControlledIPPools")
	}

	return nil
}
