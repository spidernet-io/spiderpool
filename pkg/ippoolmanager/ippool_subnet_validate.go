// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (im *ipPoolManager) validateCreateIPPoolAndUpdateSubnetFreeIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	if errs := im.validateCreateIPPool(ctx, ipPool); len(errs) != 0 {
		return errs
	}
	if !im.config.EnableSpiderSubnet {
		return nil
	}

	subnet, err := im.validateSubnetControllerExist(ctx, ipPool)
	if err != nil {
		return field.ErrorList{err}
	}
	if err := im.validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx, subnet, ipPool); err != nil {
		return field.ErrorList{err}
	}
	if err := im.removeSubnetFreeIPs(ctx, subnet, ipPool); err != nil {
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

	subnet, err := im.validateSubnetControllerExist(ctx, newIPPool)
	if err != nil {
		return field.ErrorList{err}
	}
	if err := im.validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx, subnet, newIPPool); err != nil {
		return field.ErrorList{err}
	}
	if err := im.updateSubnetFreeIPs(ctx, subnet, oldIPPool, newIPPool); err != nil {
		return field.ErrorList{err}
	}

	return nil
}

func (im *ipPoolManager) validateDeleteIPPoolAndUpdateSubnetFreeIPs(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) field.ErrorList {
	subnet, err := im.validateSubnetControllerExist(ctx, ipPool)
	if err != nil {
		switch err.Type {
		case field.ErrorTypeForbidden:
			return nil
		case field.ErrorTypeInternal:
			return field.ErrorList{err}
		}
	}

	if err := im.addSubnetFreeIPs(ctx, subnet, ipPool); err != nil {
		return field.ErrorList{err}
	}

	return nil
}

func (im *ipPoolManager) validateSubnetControllerExist(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) (*spiderpoolv1.SpiderSubnet, *field.Error) {
	subnetList, err := im.subnetManager.ListSubnets(ctx)
	if err != nil {
		return nil, field.InternalError(subnetField, err)
	}

	for _, subnet := range subnetList.Items {
		if subnet.Spec.Subnet == ipPool.Spec.Subnet {
			return &subnet, nil
		}
	}

	return nil, field.Forbidden(
		subnetField,
		fmt.Sprintf("orphan IPPool, must be controlled by Subnet with the same 'spec.subnet' %s", ipPool.Spec.Subnet),
	)
}

func (im *ipPoolManager) validateSubnetTotalIPsContainsIPPoolTotalIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	poolTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	subnetTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
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

func (im *ipPoolManager) removeSubnetFreeIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	poolTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	freeIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, subnet.Status.FreeIPs)
	if err != nil {
		return field.InternalError(freeIPsField, err)
	}

	notFreeIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, freeIPs)
	if len(notFreeIPs) > 0 {
		ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*ipPool.Spec.IPVersion, notFreeIPs)
		return field.Forbidden(
			ipsField,
			fmt.Sprintf("add some IP ranges %v that are not free in controller Subnet %s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ranges, subnet.Name),
		)
	}

	newFreeIPs := spiderpoolip.IPsDiffSet(freeIPs, poolTotalIPs)
	ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*subnet.Spec.IPVersion, newFreeIPs)
	subnet.Status.FreeIPs = ranges

	freeIPCount := int64(len(newFreeIPs))
	subnet.Status.FreeIPCount = &freeIPCount
	if err := im.subnetManager.UpdateSubnetStatusOnce(ctx, subnet); err != nil {
		return field.InternalError(freeIPsField, err)
	}

	return nil
}

func (im *ipPoolManager) updateSubnetFreeIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet, oldIPPool, newIPPool *spiderpoolv1.SpiderIPPool) *field.Error {
	oldPoolTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*oldIPPool.Spec.IPVersion, oldIPPool.Spec.IPs, oldIPPool.Spec.ExcludeIPs)
	newPoolTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*newIPPool.Spec.IPVersion, newIPPool.Spec.IPs, newIPPool.Spec.ExcludeIPs)
	addedIPs := spiderpoolip.IPsDiffSet(newPoolTotalIPs, oldPoolTotalIPs)
	freeIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, subnet.Status.FreeIPs)
	if err != nil {
		return field.InternalError(freeIPsField, err)
	}

	notFreeIPs := spiderpoolip.IPsDiffSet(addedIPs, freeIPs)
	if len(notFreeIPs) > 0 {
		ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*newIPPool.Spec.IPVersion, notFreeIPs)
		return field.Forbidden(
			ipsField,
			fmt.Sprintf("add some IP ranges %v that are not free in controller Subnet %s, total IP addresses of an IPPool are jointly determined by 'spec.ips' and 'spec.excludeIPs'", ranges, subnet.Name),
		)
	}

	reducedIPs := spiderpoolip.IPsDiffSet(oldPoolTotalIPs, newPoolTotalIPs)
	newFreeIPs := spiderpoolip.IPsDiffSet(spiderpoolip.IPsUnionSet(freeIPs, reducedIPs), addedIPs)
	ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*subnet.Spec.IPVersion, newFreeIPs)
	subnet.Status.FreeIPs = ranges

	freeIPCount := int64(len(newFreeIPs))
	subnet.Status.FreeIPCount = &freeIPCount
	if err := im.subnetManager.UpdateSubnetStatusOnce(ctx, subnet); err != nil {
		return field.InternalError(freeIPsField, err)
	}

	return nil
}

func (im *ipPoolManager) addSubnetFreeIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	freeIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, subnet.Status.FreeIPs)
	if err != nil {
		return field.InternalError(freeIPsField, err)
	}

	poolTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	subnetTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	validIPs := spiderpoolip.IPsIntersectionSet(poolTotalIPs, subnetTotalIPs)

	newFreeIPs := spiderpoolip.IPsUnionSet(freeIPs, validIPs)
	ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*subnet.Spec.IPVersion, newFreeIPs)
	subnet.Status.FreeIPs = ranges

	freeIPCount := int64(len(newFreeIPs))
	subnet.Status.FreeIPCount = &freeIPCount
	if err := im.subnetManager.UpdateSubnetStatusOnce(ctx, subnet); err != nil {
		return field.InternalError(freeIPsField, err)
	}

	return nil
}
