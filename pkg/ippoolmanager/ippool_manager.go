// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ipPoolManager struct {
	config         *IPPoolManagerConfig
	freeIPsLimiter limiter.Limiter
	client         client.Client
	runtimeMgr     ctrl.Manager
	podManager     podmanager.PodManager
	rIPManager     reservedipmanager.ReservedIPManager
	subnetManager  subnetmanagertypes.SubnetManager

	// leader only serves for Spiderpool controller SpiderIPPool informer, it will be set by SetupInformer
	leader election.SpiderLeaseElector
}

func NewIPPoolManager(c *IPPoolManagerConfig, mgr ctrl.Manager, podManager podmanager.PodManager, rIPManager reservedipmanager.ReservedIPManager) (ippoolmanagertypes.IPPoolManager, error) {
	if c == nil {
		return nil, errors.New("ippool manager config must be specified")
	}
	if mgr == nil {
		return nil, errors.New("k8s manager must be specified")
	}
	if podManager == nil {
		return nil, errors.New("pod manager must be specified")
	}
	if rIPManager == nil {
		return nil, errors.New("reserved IP manager must be specified")
	}

	maxQueueSize := 1000
	maxWaitTime := 5 * time.Second
	freeIPsLimiter := limiter.NewLimiter(limiter.LimiterConfig{
		MaxQueueSize: &maxQueueSize,
		MaxWaitTime:  &maxWaitTime,
	})

	poolMgr := &ipPoolManager{
		config:         c,
		freeIPsLimiter: freeIPsLimiter,
		client:         mgr.GetClient(),
		runtimeMgr:     mgr,
		podManager:     podManager,
		rIPManager:     rIPManager,
	}

	return poolMgr, nil
}

func (im *ipPoolManager) Start(ctx context.Context) error {
	return im.freeIPsLimiter.Start(ctx)
}

func (im *ipPoolManager) InjectSubnetManager(subnetManager subnetmanagertypes.SubnetManager) {
	if subnetManager == nil {
		return
	}

	im.subnetManager = subnetManager
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

func (im *ipPoolManager) AllocateIP(ctx context.Context, poolName, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, *spiderpoolv1.SpiderIPPool, error) {
	var ipConfig *models.IPConfig
	var usedIPPool *spiderpoolv1.SpiderIPPool
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		ipPool, err := im.GetIPPoolByName(ctx, poolName)
		if err != nil {
			return nil, nil, err
		}

		allocatedIP, err := im.genRandomIP(ctx, ipPool)
		if err != nil {
			return nil, nil, err
		}

		if ipPool.Status.AllocatedIPs == nil {
			ipPool.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
		}

		podTopController, err := im.podManager.GetPodTopController(ctx, pod)
		if err != nil {
			return nil, nil, err
		}
		allocation := spiderpoolv1.PoolIPAllocation{
			ContainerID:         containerID,
			NIC:                 nic,
			Node:                pod.Spec.NodeName,
			Namespace:           pod.Namespace,
			Pod:                 pod.Name,
			OwnerControllerType: podTopController.Kind,
		}
		allocation.OwnerControllerName = podTopController.Name

		ipPool.Status.AllocatedIPs[allocatedIP.String()] = allocation

		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int64)
		}

		*ipPool.Status.AllocatedIPCount++
		if *ipPool.Status.AllocatedIPCount > int64(im.config.MaxAllocatedIPs) {
			return nil, nil, fmt.Errorf("%w, threshold of IP allocations(<=%d) for IPPool %s exceeded", constant.ErrIPUsedOut, im.config.MaxAllocatedIPs, poolName)
		}

		usedIPPool = ipPool
		ipConfig, err = genResIPConfig(allocatedIP, &ipPool.Spec, nic, poolName)
		if err != nil {
			return nil, nil, err
		}

		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return nil, nil, err
			}
			if i == im.config.MaxConflictRetries {
				return nil, nil, fmt.Errorf("%w, failed for %d times, failed to allocate IP from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, poolName)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return ipConfig, usedIPPool, nil
}

func (im *ipPoolManager) genRandomIP(ctx context.Context, ipPool *spiderpoolv1.SpiderIPPool) (net.IP, error) {
	rIPList, err := im.rIPManager.ListReservedIPs(ctx)
	if err != nil {
		return nil, err
	}
	reservedIPs, err := reservedipmanager.AssembleReservedIPs(*ipPool.Spec.IPVersion, rIPList)
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

	return availableIPs[rand.Int()%len(availableIPs)], nil
}

func (im *ipPoolManager) ReleaseIP(ctx context.Context, poolName string, ipAndCIDs []types.IPAndCID) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
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

		needToRelease := false
		for _, e := range ipAndCIDs {
			if a, ok := ipPool.Status.AllocatedIPs[e.IP]; ok {
				if a.ContainerID == e.ContainerID {
					delete(ipPool.Status.AllocatedIPs, e.IP)
					*ipPool.Status.AllocatedIPCount--
					needToRelease = true
				}
			}
		}

		if !needToRelease {
			return nil
		}

		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == im.config.MaxConflictRetries {
				return fmt.Errorf("%w, failed for %d times, failed to release IP %+v from IPPool %s", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, ipAndCIDs, poolName)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

// TODO(iiiceoo): Refactor
func (im *ipPoolManager) CheckVlanSame(ctx context.Context, poolNameList []string) (map[types.Vlan][]string, bool, error) {
	vlanToPools := map[types.Vlan][]string{}
	for _, poolName := range poolNameList {
		ipPool, err := im.GetIPPoolByName(ctx, poolName)
		if err != nil {
			return nil, false, err
		}

		vlanToPools[*ipPool.Spec.Vlan] = append(vlanToPools[*ipPool.Spec.Vlan], poolName)
	}

	if len(vlanToPools) > 1 {
		return vlanToPools, false, nil
	}

	return vlanToPools, true, nil
}

func (im *ipPoolManager) RemoveFinalizer(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	if !controllerutil.ContainsFinalizer(pool, constant.SpiderFinalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(pool, constant.SpiderFinalizer)
	err := im.client.Update(ctx, pool)
	if nil != err {
		return err
	}

	return nil
}

// UpdateAllocatedIPs serves for StatefulSet pod re-create
func (im *ipPoolManager) UpdateAllocatedIPs(ctx context.Context, containerID string, pod *corev1.Pod, oldIPConfig models.IPConfig) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= im.config.MaxConflictRetries; i++ {
		pool, err := im.GetIPPoolByName(ctx, oldIPConfig.IPPool)
		if nil != err {
			return err
		}

		// switch CIDR to IP
		singleIP, _, _ := strings.Cut(*oldIPConfig.Address, "/")
		if allocation, ok := pool.Status.AllocatedIPs[singleIP]; ok {
			if allocation.ContainerID == containerID {
				return nil
			}
		}

		// basically, we just need to update ContainerID and Node.
		pool.Status.AllocatedIPs[singleIP] = spiderpoolv1.PoolIPAllocation{
			ContainerID:         containerID,
			NIC:                 *oldIPConfig.Nic,
			Node:                pod.Spec.NodeName,
			Namespace:           pod.Namespace,
			Pod:                 pod.Name,
			OwnerControllerType: constant.KindStatefulSet,
		}

		err = im.client.Status().Update(ctx, pool)
		if nil != err {
			if !apierrors.IsConflict(err) {
				return err
			}

			if i == im.config.MaxConflictRetries {
				return fmt.Errorf("%w, failed for %d times, failed to re-allocate StatefulSet pod '%s/%s' SpiderIPPool IP '%s'", constant.ErrRetriesExhausted, im.config.MaxConflictRetries, pod.Namespace, pod.Name, singleIP)
			}

			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * im.config.ConflictRetryUnitTime)
			continue
		}
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

func (im *ipPoolManager) DeleteIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	err := im.client.Delete(ctx, pool)
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

// ScaleIPPoolWithIPs will expand or shrink the IPPool with the given action.
// Notice: we shouldn't get retries in this method and the upper level calling function will requeue the workqueue once we return an error,
func (im *ipPoolManager) ScaleIPPoolWithIPs(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, ipRanges []string, action ippoolmanagertypes.ScaleAction, desiredIPNum int) error {
	log := logutils.FromContext(ctx)

	var err error

	// filter out exclude IPs.
	currentIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return fmt.Errorf("failed to assemble Total IP addresses: %v", err)
	}

	if len(currentIPs) == desiredIPNum {
		log.Sugar().Debugf("IPPool '%s' already has desired IP number '%d' IPs, non need to scale it", pool.Name, desiredIPNum)
		return nil
	}

	if action == ippoolmanagertypes.ScaleUpIP {
		pool.Spec.IPs = append(pool.Spec.IPs, ipRanges...)
		sortedIPRanges, err := spiderpoolip.MergeIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
		if nil != err {
			return fmt.Errorf("failed to merge IP ranges '%v', error: %v", pool.Spec.IPs, err)
		}

		log.With(zap.String("ScaleUpIP", fmt.Sprintf("add IPs '%v'", ipRanges))).
			Sugar().Infof("update IPPool '%s' IPs from '%v' to '%v'", pool.Name, pool.Spec.IPs, sortedIPRanges)
		pool.Spec.IPs = sortedIPRanges
	} else {
		discardedIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, ipRanges)
		if nil != err {
			return fmt.Errorf("failed to parse IP ranges '%v', error: %v", ipRanges, err)
		}

		// the original IPPool.Spec.IPs
		totalIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
		if nil != err {
			return fmt.Errorf("failed to parse IP ranges '%v', error: %v", pool.Spec.IPs, err)
		}

		sortedIPRanges, err := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, spiderpoolip.IPsDiffSet(totalIPs, discardedIPs))
		if nil != err {
			return fmt.Errorf("failed to convert IPs '%v' to IP ranges, error: %v", ipRanges, err)
		}

		log.With(zap.String("ScaleDownIP", fmt.Sprintf("discard IPs '%v'", ipRanges))).
			Sugar().Infof("update IPPool '%s' IPs from '%v' to '%v'", pool.Name, pool.Spec.IPs, sortedIPRanges)
		pool.Spec.IPs = sortedIPRanges
	}

	err = im.client.Update(ctx, pool)
	if nil != err {
		return fmt.Errorf("failed to update IPPool '%s', error: %w", pool.Name, err)
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
