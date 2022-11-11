// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

func (im *ipPoolManager) validateCreateIPPoolAndUpdateSubnetFreeIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	if errs := im.validateCreateIPPool(ctx, ipPool); len(errs) != 0 {
		return errs
	}
	if !im.config.EnableSpiderSubnet {
		return nil
	}

	subnet, err := im.validateSubnetControllerExist(ctx, ipPool, false)
	if err != nil {
		return field.ErrorList{err}
	}
	if err := validateSubnetTotalIPsContainsIPPoolTotalIPs(subnet, ipPool); err != nil {
		return field.ErrorList{err}
	}

	return nil
}

func (im *ipPoolManager) validateUpdateIPPoolAndUpdateSubnetFreeIPs(ctx context.Context, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	if errs := im.validateUpdateIPPool(ctx, oldIPPool, newIPPool); len(errs) != 0 {
		return errs
	}
	if !im.config.EnableSpiderSubnet {
		return nil
	}

	if newIPPool.DeletionTimestamp != nil && !controllerutil.ContainsFinalizer(newIPPool, constant.SpiderFinalizer) {
		return im.validateDeleteIPPoolAndUpdateSubnetFreeIPs(ctx, newIPPool)
	}

	subnet, err := im.validateSubnetControllerExist(ctx, newIPPool, false)
	if err != nil {
		return field.ErrorList{err}
	}
	if err := validateSubnetTotalIPsContainsIPPoolTotalIPs(subnet, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	return nil
}

func (im *ipPoolManager) validateDeleteIPPoolAndUpdateSubnetFreeIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	subnet, err := im.validateSubnetControllerExist(ctx, ipPool, true)
	if err != nil {
		return field.ErrorList{err}
	}
	if subnet == nil {
		return nil
	}

	return nil
}

func (im *ipPoolManager) validateSubnetControllerExist(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool, terminaing bool) (*spiderpoolv1.SpiderSubnet, *field.Error) {
	subnetList, err := im.subnetManager.ListSubnets(ctx)
	if err != nil {
		return nil, field.InternalError(subnetField, err)
	}

	for _, subnet := range subnetList.Items {
		if subnet.Spec.Subnet == ipPool.Spec.Subnet {
			if !terminaing && subnet.DeletionTimestamp != nil {
				return nil, field.Forbidden(
					subnetField,
					fmt.Sprintf("cannot update IPPool that controlled by terminating Subnet %s", subnet.Name),
				)
			}
			return &subnet, nil
		}
	}

	if !terminaing {
		return nil, field.Forbidden(
			subnetField,
			fmt.Sprintf("orphan IPPool, must be controlled by Subnet with the same 'spec.subnet' %s", ipPool.Spec.Subnet),
		)
	}

	return nil, nil
}

func validateSubnetTotalIPsContainsIPPoolTotalIPs(subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, err)
	}
	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return field.InternalError(ipsField, err)
	}

	outIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, subnetTotalIPs)
	if len(outIPs) > 0 {
		ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*ipPool.Spec.IPVersion, outIPs)
		return field.Forbidden(
			ipsField,
			fmt.Sprintf("add some IP ranges %v that are not contained in controller Subnet %s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ranges, subnet.Name),
		)
	}

	return nil
}
