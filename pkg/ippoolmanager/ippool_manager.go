// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/utils/retry"
)

type IPPoolManager interface {
	GetIPPoolByName(ctx context.Context, poolName string, cached bool) (*spiderpoolv2beta1.SpiderIPPool, error)
	ListIPPools(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderIPPoolList, error)
	AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod) (*models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error
	UpdateAllocatedIPs(ctx context.Context, poolName string, ipAndCIDs []types.IPAndUID) error
}

type ipPoolManager struct {
	config     IPPoolManagerConfig
	client     client.Client
	apiReader  client.Reader
	rIPManager reservedipmanager.ReservedIPManager
}

func NewIPPoolManager(config IPPoolManagerConfig, client client.Client, apiReader client.Reader, rIPManager reservedipmanager.ReservedIPManager) (IPPoolManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}
	if rIPManager == nil {
		return nil, fmt.Errorf("reserved-IP manager %w", constant.ErrMissingRequiredParam)
	}

	return &ipPoolManager{
		config:     setDefaultsForIPPoolManagerConfig(config),
		client:     client,
		apiReader:  apiReader,
		rIPManager: rIPManager,
	}, nil
}

func (im *ipPoolManager) GetIPPoolByName(ctx context.Context, poolName string, cached bool) (*spiderpoolv2beta1.SpiderIPPool, error) {
	reader := im.apiReader
	if cached == constant.UseCache {
		reader = im.client
	}

	var ipPool spiderpoolv2beta1.SpiderIPPool
	if err := reader.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool); err != nil {
		return nil, err
	}

	return &ipPool, nil
}

func (im *ipPoolManager) ListIPPools(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderIPPoolList, error) {
	reader := im.apiReader
	if cached == constant.UseCache {
		reader = im.client
	}

	var ipPoolList spiderpoolv2beta1.SpiderIPPoolList
	if err := reader.List(ctx, &ipPoolList, opts...); err != nil {
		return nil, err
	}

	return &ipPoolList, nil
}

func (im *ipPoolManager) AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod) (*models.IPConfig, error) {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	var ipConfig *models.IPConfig
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)
		logger.Debug("Re-get IPPool for IP allocation")
		ipPool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}

		logger.Debug("Generate a random IP address")
		allocatedIP, err := im.genRandomIP(ctx, nic, ipPool, pod)
		if err != nil {
			return err
		}

		resourceVersion := ipPool.ResourceVersion
		logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).
			Sugar().Debugf("Try to update the allocation status of IPPool using random IP %s", allocatedIP)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if apierrors.IsConflict(err) {
				metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
				logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).Warn("An conflict occurred when updating the status of IPPool")
			}
			return err
		}
		ipConfig = convert.GenIPConfigResult(allocatedIP, nic, ipPool)

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to allocate IP from IPPool %s", constant.ErrRetriesExhausted, steps, poolName)
		}

		return nil, err
	}

	return ipConfig, nil
}

func (im *ipPoolManager) genRandomIP(ctx context.Context, nic string, ipPool *spiderpoolv2beta1.SpiderIPPool, pod *corev1.Pod) (net.IP, error) {
	reservedIPs, err := im.rIPManager.AssembleReservedIPs(ctx, *ipPool.Spec.IPVersion)
	if err != nil {
		return nil, err
	}

	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
	if err != nil {
		return nil, err
	}

	var used []string
	for ip := range allocatedRecords {
		used = append(used, ip)
	}
	usedIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, used)
	if err != nil {
		return nil, err
	}

	totalIPs, err := spiderpoolip.AssembleTotalIPs(*ipPool.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return nil, err
	}

	availableIPs := spiderpoolip.IPsDiffSet(totalIPs, append(reservedIPs, usedIPs...), false)
	if len(availableIPs) == 0 {
		return nil, constant.ErrIPUsedOut
	}
	resIP := availableIPs[0]

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		return nil, err
	}

	if allocatedRecords == nil {
		allocatedRecords = spiderpoolv2beta1.PoolIPAllocations{}
	}
	allocatedRecords[resIP.String()] = spiderpoolv2beta1.PoolIPAllocation{
		NIC:            nic,
		NamespacedName: key,
		PodUID:         string(pod.UID),
	}

	data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
	if err != nil {
		return nil, err
	}
	ipPool.Status.AllocatedIPs = data

	if ipPool.Status.AllocatedIPCount == nil {
		ipPool.Status.AllocatedIPCount = new(int64)
	}

	*ipPool.Status.AllocatedIPCount++
	if *ipPool.Status.AllocatedIPCount > int64(*im.config.MaxAllocatedIPs) {
		return nil, fmt.Errorf("%w, threshold of IP records(<=%d) for IPPool %s exceeded", constant.ErrIPUsedOut, im.config.MaxAllocatedIPs, ipPool.Name)
	}

	return resIP, nil
}

func (im *ipPoolManager) ReleaseIP(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)
		logger.Debug("Re-get IPPool for IP release")
		ipPool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}

		allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
		if err != nil {
			return err
		}

		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int64)
		}

		release := false
		for _, iu := range ipAndUIDs {
			if record, ok := allocatedRecords[iu.IP]; ok {
				if record.PodUID == iu.UID {
					delete(allocatedRecords, iu.IP)
					*ipPool.Status.AllocatedIPCount--
					release = true
				}
			}
		}

		if !release {
			return nil
		}

		data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
		if err != nil {
			return err
		}
		ipPool.Status.AllocatedIPs = data

		resourceVersion := ipPool.ResourceVersion
		logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).
			Sugar().Debugf("Try to clean the IP allocation records of IPPool with IP addresses %+v", ipAndUIDs)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if apierrors.IsConflict(err) {
				metric.IpamReleaseUpdateIPPoolConflictCounts.Add(ctx, 1)
				logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).Warn("An conflict occurred when cleaning the IP allocation records of IPPool")
			}
			return err
		}

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to release IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, steps, ipAndUIDs, poolName)
		}
		return err
	}

	return nil
}

func (im *ipPoolManager) UpdateAllocatedIPs(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)

		ipPool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}

		allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
		if err != nil {
			return err
		}

		recreate := false
		for _, iu := range ipAndUIDs {
			if record, ok := allocatedRecords[iu.IP]; ok {
				if record.PodUID != iu.UID {
					record.PodUID = iu.UID
					allocatedRecords[iu.IP] = record
					recreate = true
				}
			}
		}

		if !recreate {
			return nil
		}

		data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
		if err != nil {
			return err
		}
		ipPool.Status.AllocatedIPs = data

		resourceVersion := ipPool.ResourceVersion
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if apierrors.IsConflict(err) {
				metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
				logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).Warn("An conflict occurred when updating the status of IPPool")
			}
			return err
		}

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to re-allocate the IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, steps, ipAndUIDs, poolName)
		}
		return err
	}

	return nil
}
