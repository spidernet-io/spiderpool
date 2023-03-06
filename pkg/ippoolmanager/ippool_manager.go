// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

type IPPoolManager interface {
	GetIPPoolByName(ctx context.Context, poolName string) (*spiderpoolv1.SpiderIPPool, error)
	ListIPPools(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderIPPoolList, error)
	AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod) (*models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error
	UpdateAllocatedIPs(ctx context.Context, poolName string, ipAndCIDs []types.IPAndUID) error
	UpdateDesiredIPNumber(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, ipNum int) error
}

type ipPoolManager struct {
	config     IPPoolManagerConfig
	client     client.Client
	rIPManager reservedipmanager.ReservedIPManager
}

func NewIPPoolManager(config IPPoolManagerConfig, client client.Client, rIPManager reservedipmanager.ReservedIPManager) (IPPoolManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if rIPManager == nil {
		return nil, fmt.Errorf("reserved-IP manager %w", constant.ErrMissingRequiredParam)
	}

	return &ipPoolManager{
		config:     setDefaultsForIPPoolManagerConfig(config),
		client:     client,
		rIPManager: rIPManager,
	}, nil
}

func (im *ipPoolManager) GetIPPoolByName(ctx context.Context, poolName string) (*spiderpoolv1.SpiderIPPool, error) {
	var ipPool spiderpoolv1.SpiderIPPool
	if err := im.client.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool); err != nil {
		return nil, err
	}

	return &ipPool, nil
}

func (im *ipPoolManager) ListIPPools(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderIPPoolList, error) {
	var ipPoolList spiderpoolv1.SpiderIPPoolList
	if err := im.client.List(ctx, &ipPoolList, opts...); err != nil {
		return nil, err
	}

	return &ipPoolList, nil
}

func (im *ipPoolManager) AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod) (*models.IPConfig, error) {
	logger := logutils.FromContext(ctx)

	var ipConfig *models.IPConfig
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		logger := logger.With(zap.Int("times", i+1))
		logger.Sugar().Debugf("Re-get IPPool %s for IP allocation", poolName)
		ipPool, err := im.GetIPPoolByName(ctx, poolName)
		if err != nil {
			return nil, err
		}

		logger.Debug("Generate a random IP address")
		allocatedIP, err := im.genRandomIP(ctx, nic, ipPool, pod)
		if err != nil {
			return nil, err
		}

		logger.Sugar().Debugf("Try to update the allocation status of IPPool %s with random IP %s", ipPool.Name, allocatedIP)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return nil, err
			}

			metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
			if i == im.config.MaxConflictRetries {
				return nil, fmt.Errorf("%w (%d times), failed to allocate IP from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipPool.Name)
			}

			interval := time.Duration(r.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime
			logger.Sugar().Debugf("An conflict occurred when updating the status of the IPPool %s, it will be retried in %s", ipPool.Name, interval)
			time.Sleep(interval)
			continue
		}

		ipConfig = convert.GenIPConfigResult(allocatedIP, nic, ipPool)
		break
	}

	return ipConfig, nil
}

func (im *ipPoolManager) genRandomIP(ctx context.Context, nic string, ipPool *spiderpoolv1.SpiderIPPool, pod *corev1.Pod) (net.IP, error) {
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
	allocatedRecords[resIP.String()] = spiderpoolv1.PoolIPAllocation{
		NIC:       nic,
		Namespace: pod.Namespace,
		Pod:       pod.Name,
		UID:       string(pod.UID),
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

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		logger := logger.With(zap.Int("times", i+1))
		logger.Sugar().Debugf("Re-get IPPool %s for IP release", poolName)
		ipPool, err := im.GetIPPoolByName(ctx, poolName)
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
				if record.UID == iu.UID {
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

		logger.Sugar().Debugf("Try to clean the allocation records of IPPool %s with IP addresses %+v", ipPool.Name, ipAndUIDs)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}

			metric.IpamReleaseUpdateIPPoolConflictCounts.Add(ctx, 1)
			if i == im.config.MaxConflictRetries {
				return fmt.Errorf("%w (%d times), failed to release IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipAndUIDs, poolName)
			}

			interval := time.Duration(r.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime
			logger.Sugar().Debugf("An conflict occurred when releasing form the IPPool %s, it will be retried in %s", ipPool.Name, interval)

			time.Sleep(interval)
			continue
		}
		break
	}

	return nil
}

func (im *ipPoolManager) UpdateAllocatedIPs(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		ipPool, err := im.GetIPPoolByName(ctx, poolName)
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
				if record.UID != iu.UID {
					record.UID = iu.UID
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

		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}

			metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
			if i == im.config.MaxConflictRetries {
				return fmt.Errorf("%w (%d times), failed to re-allocate the IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipAndUIDs, poolName)
			}

			time.Sleep(time.Duration(r.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

func (im *ipPoolManager) UpdateDesiredIPNumber(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, ipNum int) error {
	if pool.Status.AutoDesiredIPCount == nil {
		pool.Status.AutoDesiredIPCount = new(int64)
	} else {
		if *pool.Status.AutoDesiredIPCount == int64(ipNum) {
			return nil
		}
	}

	*pool.Status.AutoDesiredIPCount = int64(ipNum)
	err := im.client.Status().Update(ctx, pool)
	if nil != err {
		return fmt.Errorf("failed to update IPPool '%s' auto desired IP count to %d : %v", pool.Name, ipNum, err)
	}

	return nil
}
