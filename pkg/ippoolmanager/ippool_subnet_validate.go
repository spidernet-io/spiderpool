// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/event"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
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
	if err := im.updateSubnetStatus(ctx, creating, subnet, ipPool); err != nil {
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

	totalIPsChange := false
	if !reflect.DeepEqual(newIPPool.Spec.IPs, oldIPPool.Spec.IPs) ||
		!reflect.DeepEqual(newIPPool.Spec.ExcludeIPs, oldIPPool.Spec.ExcludeIPs) {
		totalIPsChange = true
	}

	if totalIPsChange {
		if err := im.updateSubnetStatus(ctx, updating, subnet, newIPPool); err != nil {
			return field.ErrorList{err}
		}
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

	if err := im.updateSubnetStatus(ctx, deleting, subnet, ipPool); err != nil {
		return field.ErrorList{err}
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

type Operation int

const (
	creating Operation = iota
	updating
	deleting
)

func (im *ipPoolManager) updateSubnetStatus(ctx context.Context, op Operation, subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) *field.Error {
	logger := logutils.FromContext(ctx)

	_, err := im.freeIPsLimiter.AcquireTicket(ctx, subnet.Name)
	if err != nil {
		logger.Sugar().Errorf("Failed to queue correctly: %v", err)
	} else {
		defer im.freeIPsLimiter.ReleaseTicket(ctx, subnet.Name)
	}

	var eventReason, eventAction string

	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		var err error
		if i != 0 {
			subnet, err = im.subnetManager.GetSubnetByName(ctx, subnet.Name)
			if err != nil {
				return field.InternalError(controlledIPPoolsField, err)
			}
		}

		needToUpdate := true
		switch op {
		case creating, updating:
			eventReason = constant.EventReasonScaleIPPool

			if eventAction, err = updateControlledIPPoolIPs(subnet, ipPool); err != nil {
				return field.InternalError(controlledIPPoolsField, err)
			}
		case deleting:
			eventReason = constant.EventReasonDeleteIPPool

			needToUpdate, eventAction, err = removeControlledIPPoolIPs(subnet, ipPool)
			if err != nil {
				return field.InternalError(controlledIPPoolsField, err)
			}
		}

		if !needToUpdate {
			return nil
		}

		if err := im.client.Status().Update(ctx, subnet); err != nil {
			if !apierrors.IsConflict(err) {
				return field.InternalError(controlledIPPoolsField, err)
			}
			if i == im.config.MaxConflictRetries {
				event.EventRecorder.Event(subnet, corev1.EventTypeWarning, eventReason, fmt.Sprintf("%s unsuccessfully", eventAction))
				return field.InternalError(
					controlledIPPoolsField,
					fmt.Errorf("%w, failed for %d times, failed to update the IP addresses of the controlled IPPool of the Subnet %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, subnet.Name),
				)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime)
			continue
		}
		event.EventRecorder.Event(subnet, corev1.EventTypeNormal, eventReason, fmt.Sprintf("%s successfully", eventAction))
		break
	}

	return nil
}

func updateControlledIPPoolIPs(subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) (eventAction string, err error) {
	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return
	}
	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return
	}
	validIPs := spiderpoolip.IPsIntersectionSet(poolTotalIPs, subnetTotalIPs)
	ranges, err := spiderpoolip.ConvertIPsToIPRanges(*subnet.Spec.IPVersion, validIPs)
	if err != nil {
		return
	}

	var oldPoolTotalIPs []net.IP
	if subnet.Status.ControlledIPPools == nil {
		subnet.Status.ControlledIPPools = spiderpoolv1.PoolIPPreAllocations{}
	} else if pool, ok := subnet.Status.ControlledIPPools[ipPool.Name]; ok {
		oldPoolTotalIPs, err = spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, pool.IPs)
		if err != nil {
			return
		}
	}
	subnet.Status.ControlledIPPools[ipPool.Name] = spiderpoolv1.PoolIPPreAllocation{IPs: ranges}

	if subnet.Status.AllocatedIPCount == nil {
		subnet.Status.AllocatedIPCount = new(int64)
	}
	delta := int64(len(poolTotalIPs) - len(oldPoolTotalIPs))
	*subnet.Status.AllocatedIPCount += delta

	if delta > 0 {
		eventAction = fmt.Sprintf("IPPool '%s' requests to generate '%d' IPs", ipPool.Name, delta)
	} else if delta < 0 {
		eventAction = fmt.Sprintf("IPPool '%s' requests to return '%d' IPs", ipPool.Name, -1*delta)
	} else {
		if len(poolTotalIPs) == 0 {
			eventAction = fmt.Sprintf("Creating an empty IPPool '%s'", ipPool.Name)
		} else {
			// TODO(Icarus9913): refactor this case description
			// rare case: update IPPool totalIPs, its count is same with the old totalIP counts
			eventAction = fmt.Sprintf("update IPPool '%s' totalIPs from '%s' to '%s'", ipPool.Name, oldPoolTotalIPs, ranges)
		}
	}
	return
}

func removeControlledIPPoolIPs(subnet *spiderpoolv1.SpiderSubnet, ipPool *spiderpoolv1.SpiderIPPool) (needToUpdate bool, eventAction string, err error) {
	if subnet.Status.ControlledIPPools == nil {
		return
	}

	pool, ok := subnet.Status.ControlledIPPools[ipPool.Name]
	if !ok {
		return
	}
	poolTotalIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, pool.IPs)
	if err != nil {
		return
	}
	delete(subnet.Status.ControlledIPPools, ipPool.Name)
	*subnet.Status.AllocatedIPCount -= int64(len(poolTotalIPs))

	eventAction = fmt.Sprintf("IPPool '%s' requests to return '%d' IPs", ipPool.Name, len(poolTotalIPs))

	return true, eventAction, nil
}
