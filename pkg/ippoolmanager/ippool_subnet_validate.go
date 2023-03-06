// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

func (iw *IPPoolWebhook) validateCreateIPPoolWhileEnableSpiderSubnet(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
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

func (iw *IPPoolWebhook) validateUpdateIPPoolWhileEnableSpiderSubnet(ctx context.Context, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
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

func (iw *IPPoolWebhook) validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	owner := metav1.GetControllerOf(ipPool)
	if owner == nil {
		return field.Forbidden(
			subnetField,
			fmt.Sprintf("orphan IPPool, must be controlled by Subnet with the same 'spec.subnet' %s", ipPool.Spec.Subnet),
		)
	}

	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, fmt.Errorf("failed to assemble the total IP addresses of the IPPool %s: %v", ipPool.Name, err))
	}
	if len(poolTotalIPs) == 0 {
		return nil
	}

	var subnet spiderpoolv1.SpiderSubnet
	if err := iw.Client.Get(ctx, apitypes.NamespacedName{Name: owner.Name}, &subnet); err != nil {
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

	return nil
}
