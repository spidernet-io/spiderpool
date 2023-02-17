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
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type IPPoolManager interface {
	GetIPPoolByName(ctx context.Context, poolName string) (*spiderpoolv1.SpiderIPPool, error)
	ListIPPools(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderIPPoolList, error)
	AllocateIP(ctx context.Context, poolName, containerID, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndCIDs []types.IPAndCID) error
	UpdateAllocatedIPs(ctx context.Context, poolName string, ipAndCIDs []types.IPAndCID) error
	DeleteAllIPPools(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, opts ...client.DeleteAllOfOption) error
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

func (im *ipPoolManager) AllocateIP(ctx context.Context, poolName, containerID, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, error) {
	logger := logutils.FromContext(ctx)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var ipConfig *models.IPConfig
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		logger := logger.With(zap.Int("times", i+1))
		logger.Sugar().Debugf("Re-get IPPool %s for IP allocation", poolName)

		ipPool, err := im.GetIPPoolByName(ctx, poolName)
		if err != nil {
			return nil, err
		}

		logger.Debug("Generate a random IP address")
		allocatedIP, err := im.genRandomIP(ctx, ipPool)
		if err != nil {
			return nil, err
		}

		if ipPool.Status.AllocatedIPs == nil {
			ipPool.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
		}

		allocation := spiderpoolv1.PoolIPAllocation{
			ContainerID:         containerID,
			NIC:                 nic,
			Node:                pod.Spec.NodeName,
			Namespace:           pod.Namespace,
			Pod:                 pod.Name,
			OwnerControllerType: podController.Kind,
			OwnerControllerName: podController.Name,
		}

		ip := allocatedIP.String()
		ipPool.Status.AllocatedIPs[ip] = allocation

		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int64)
		}

		*ipPool.Status.AllocatedIPCount++
		if *ipPool.Status.AllocatedIPCount > int64(*im.config.MaxAllocatedIPs) {
			return nil, fmt.Errorf("%w, threshold of IP allocations(<=%d) for IPPool %s exceeded", constant.ErrIPUsedOut, im.config.MaxAllocatedIPs, ipPool.Name)
		}

		logger.Sugar().Debugf("Try to update the allocation status of IPPool %s with random IP %s", ipPool.Name, ip)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return nil, err
			}
			if i == im.config.MaxConflictRetries {
				return nil, fmt.Errorf("%w (%d times), failed to allocate IP from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipPool.Name)
			}

			interval := time.Duration(r.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime
			logger.Sugar().Debugf("An conflict occurred when updating the status of the IPPool %s, it will be retried in %s", ipPool.Name, interval)

			time.Sleep(interval)
			continue
		}

		ipConfig = genResIPConfig(allocatedIP, nic, ipPool)
		break
	}

	return ipConfig, nil
}

func (im *ipPoolManager) genRandomIP(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) (net.IP, error) {
	reservedIPs, err := im.rIPManager.AssembleReservedIPs(ctx, *ipPool.Spec.IPVersion)
	if err != nil {
		return nil, err
	}

	var used []string
	for ip := range ipPool.Status.AllocatedIPs {
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

	availableIPs := spiderpoolip.IPsDiffSet(totalIPs, append(reservedIPs, usedIPs...))
	if len(availableIPs) == 0 {
		return nil, constant.ErrIPUsedOut
	}

	return availableIPs[0], nil
}

func (im *ipPoolManager) ReleaseIP(ctx context.Context, poolName string, ipAndCIDs []types.IPAndCID) error {
	logger := logutils.FromContext(ctx)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		logger := logger.With(zap.Int("times", i+1))
		logger.Sugar().Debugf("Re-get IPPool %s for IP release", poolName)

		ipPool, err := im.GetIPPoolByName(ctx, poolName)
		if err != nil {
			return err
		}

		if ipPool.Status.AllocatedIPs == nil {
			ipPool.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
		}
		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int64)
		}

		release := false
		for _, cur := range ipAndCIDs {
			if record, ok := ipPool.Status.AllocatedIPs[cur.IP]; ok {
				if record.ContainerID == cur.ContainerID {
					delete(ipPool.Status.AllocatedIPs, cur.IP)
					*ipPool.Status.AllocatedIPCount--
					release = true
				}
			}
		}

		if !release {
			return nil
		}

		logger.Sugar().Debugf("Try to clean the allocation status of IPPool %s with IP addresses %+v", ipPool.Name, ipAndCIDs)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == im.config.MaxConflictRetries {
				return fmt.Errorf("%w (%d times), failed to release IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipAndCIDs, poolName)
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

func (im *ipPoolManager) UpdateAllocatedIPs(ctx context.Context, poolName string, ipAndCIDs []types.IPAndCID) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		ipPool, err := im.GetIPPoolByName(ctx, poolName)
		if err != nil {
			return err
		}

		recreate := false
		for _, cur := range ipAndCIDs {
			if record, ok := ipPool.Status.AllocatedIPs[cur.IP]; ok {
				if record.ContainerID == cur.ContainerID {
					continue
				}

				record.ContainerID = cur.ContainerID
				record.Node = cur.Node
				recreate = true
			}
		}

		if !recreate {
			return nil
		}

		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == im.config.MaxConflictRetries {
				return fmt.Errorf("%w (%d times), failed to re-allocate the IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipAndCIDs, poolName)
			}

			time.Sleep(time.Duration(r.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

func (im *ipPoolManager) CreateIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	err := im.client.Create(ctx, pool)
	if nil != err {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}

		return fmt.Errorf("failed to create IPPool '%s', error: %v", pool.Name, err)
	}

	return nil
}

func (im *ipPoolManager) DeleteAllIPPools(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, opts ...client.DeleteAllOfOption) error {
	err := im.client.DeleteAllOf(ctx, pool, opts...)
	if client.IgnoreNotFound(err) != nil {
		return err
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
