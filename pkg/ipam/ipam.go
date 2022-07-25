// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

type IPAM interface {
	Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error)
	Release(ctx context.Context, delArgs *models.IpamDelArgs) error
	Start(ctx context.Context) error
}

type IPAMConfig struct {
	StatuflsetIPEnable       bool
	EnableIPv4               bool
	EnableIPv6               bool
	ClusterDefaultIPv4IPPool []string
	ClusterDefaultIPv6IPPool []string
	LimiterMaxQueueSize      int
	LimiterMaxWaitTime       time.Duration
}

type ipam struct {
	ipamConfig    *IPAMConfig
	ipamLimiter   limiter.Limiter
	ipPoolManager ippoolmanager.IPPoolManager
	weManager     workloadendpointmanager.WorkloadEndpointManager
	nsManager     namespacemanager.NamespaceManager
	podManager    podmanager.PodManager
}

func NewIPAM(c *IPAMConfig, ipPoolManager ippoolmanager.IPPoolManager, weManager workloadendpointmanager.WorkloadEndpointManager, nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager) (IPAM, error) {
	if c == nil {
		return nil, errors.New("ipam config must be specified")
	}
	if ipPoolManager == nil {
		return nil, errors.New("ip pool manager must be specified")
	}
	if weManager == nil {
		return nil, errors.New("workload endpoint manager must be specified")
	}
	if nsManager == nil {
		return nil, errors.New("namespace manager must be specified")
	}
	if podManager == nil {
		return nil, errors.New("pod manager must be specified")
	}

	ipamLimiter := limiter.NewLimiter(c.LimiterMaxQueueSize, c.LimiterMaxWaitTime)
	return &ipam{
		ipamConfig:    c,
		ipamLimiter:   ipamLimiter,
		ipPoolManager: ipPoolManager,
		weManager:     weManager,
		nsManager:     nsManager,
		podManager:    podManager,
	}, nil
}

type ToBeAllocated struct {
	NIC              string
	V4PoolCandidates []string
	V6PoolCandidates []string
}

func (i *ipam) Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to allocate IP")

	pod, err := i.podManager.GetPodByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %v", *addArgs.PodName, err)
	}

	podStatus, allocatable := i.podManager.CheckPodStatus(ctx, pod)
	if !allocatable {
		return nil, fmt.Errorf("pod is %s: %w", podStatus, constant.ErrNotAllocatablePod)
	}

	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *addArgs.PodNamespace, *addArgs.PodName, *addArgs.ContainerID, *addArgs.IfName, false)
	if err != nil {
		return nil, err
	}

	addResp := &models.IpamAddResponse{}
	if allocation != nil && currently {
		logger.Sugar().Infof("Retrieve an existing IP allocation: %+v", allocation.IPs)
		addResp.Ips = convertToIPConfigs(allocation.IPs)
		// TODO(iiiceoo): Return routes or DNS
		// addResp.DNS = xxx
		// addResp.Routes = xxx

		return addResp, nil
	}

	preliminary, err := i.getPoolCandidates(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IP pool candidates: %+v", preliminary)

	if err := i.checkIPVersionEnable(ctx, preliminary); err != nil {
		return nil, err
	}

	toBeAllocatedSet, err := i.filterPoolCandidates(ctx, preliminary, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Filtered IP pool candidates: %+v", toBeAllocatedSet)

	if err := i.verifyPoolCandidates(ctx, toBeAllocatedSet); err != nil {
		return nil, err
	}
	logger.Info("All IP pool candidates valid")

	resIPs, err := i.allocateForAllInterfaces(ctx, toBeAllocatedSet, *addArgs.ContainerID, pod)
	if err != nil {
		// If there are any other errors that might have been thrown at Allocate
		// after the allocateForAllInterfaces is called, use defer.
		if len(resIPs) != 0 {
			if err := i.release(ctx, *addArgs.ContainerID, convertToIPDetails(resIPs)); err != nil {
				logger.Sugar().Warnf("Failed to roll back allocated IP: %v", err)
				return nil, err
			}
		}

		if err := i.weManager.ClearCurrentIPAllocation(ctx, *addArgs.PodNamespace, *addArgs.PodName, *addArgs.ContainerID); err != nil {
			logger.Sugar().Warnf("Failed to clear current IP allocation: %v", err)
		}

		return nil, err
	}

	addResp.Ips = resIPs
	logger.Sugar().Infof("Succeed to allocate: %+v", addResp)

	return addResp, nil
}

func (i *ipam) allocateForAllInterfaces(ctx context.Context, tt []ToBeAllocated, containerID string, pod *corev1.Pod) ([]*models.IPConfig, error) {
	// TODO(iiiceoo): Comment why containerID should be written first.
	if err := i.weManager.MarkIPAllocation(ctx, pod.Spec.NodeName, pod.Namespace, pod.Name, containerID); err != nil {
		return nil, fmt.Errorf("failed to mark IP allocation: %v", err)
	}

	var ips []*models.IPConfig
	for _, t := range tt {
		if len(t.V4PoolCandidates) != 0 {
			ipv4, err := i.allocateIPFromPoolCandidates(ctx, t.V4PoolCandidates, constant.IPv4, containerID, t.NIC, pod)
			if ipv4 != nil {
				ips = append(ips, ipv4)
			}
			if err != nil {
				return ips, err
			}
		}

		if len(t.V6PoolCandidates) != 0 {
			ipv6, err := i.allocateIPFromPoolCandidates(ctx, t.V6PoolCandidates, constant.IPv6, containerID, t.NIC, pod)
			if ipv6 != nil {
				ips = append(ips, ipv6)
			}
			if err != nil {
				return ips, err
			}
		}
	}

	anno, err := genIPAssignmentAnnotation(ips)
	if err != nil {
		return ips, err
	}

	if err := i.podManager.MergeAnnotations(ctx, pod.Namespace, pod.Name, anno); err != nil {
		return ips, fmt.Errorf("failed to merge IP assignment annotation of pod: %v", err)
	}

	return ips, nil
}

func (i *ipam) allocateIPFromPoolCandidates(ctx context.Context, poolCandidates []string, version types.IPVersion, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, error) {
	logger := logutils.FromContext(ctx)

	// TODO(iiiceoo): Comment why queue up before allocating IP.
	_, err := i.ipamLimiter.AcquireTicket(ctx, poolCandidates...)
	if err != nil {
		logger.Sugar().Errorf("Failed to queue correctly: %v", err)
	} else {
		defer i.ipamLimiter.ReleaseTicket(ctx, poolCandidates...)
	}

	var errs []error
	var ip *models.IPConfig
	for _, pool := range poolCandidates {
		var err error
		ip, err = i.ipPoolManager.AllocateIP(ctx, pool, containerID, nic, pod)
		if err != nil {
			errs = append(errs, err)
			logger.Sugar().Warnf("Failed to allocate IPv%d IP to %s from IP pool %s: %v", version, nic, pool, err)
			continue
		}
		logger.Sugar().Infof("Allocate IPv%d IP %s to %s from IP pool %s", version, *ip.Address, nic, pool)
		break
	}

	if ip == nil {
		return ip, fmt.Errorf("failed to allocate any IPv%d IP to %s from IP pools %v: %v", version, nic, poolCandidates, utilerrors.NewAggregate(errs).Error())
	}

	patch := convertToIPDetails([]*models.IPConfig{ip})
	if err := i.weManager.PatchIPAllocation(ctx, pod.Namespace, pod.Name, &spiderpoolv1.PodIPAllocation{
		ContainerID: containerID,
		IPs:         patch,
	}); err != nil {
		return ip, fmt.Errorf("failed to update IP allocation detail %+v of workload endpoint: %v", patch, err)
	}

	return ip, nil
}

func (i *ipam) getPoolCandidates(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, pod *corev1.Pod) ([]ToBeAllocated, error) {
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		return i.getPoolFromPodAnnoPools(ctx, anno, nic)
	}

	if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		t, err := i.getPoolFromPodAnnoPool(ctx, anno, nic)
		if err != nil {
			return nil, err
		}
		return []ToBeAllocated{*t}, nil
	}

	t, err := i.getPoolFromNS(ctx, pod.Namespace, nic)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return []ToBeAllocated{*t}, nil
	}

	if t := i.getPoolFromNetConf(ctx, nic, netConfV4Pool, netConfV6Pool); t != nil {
		return []ToBeAllocated{*t}, nil
	}

	t, err = i.getClusterDefaultPool(ctx, nic)
	if err != nil {
		return nil, err
	}

	return []ToBeAllocated{*t}, nil
}

func (i *ipam) getPoolFromPodAnnoPools(ctx context.Context, anno, nic string) ([]ToBeAllocated, error) {
	// TODO(iiiceoo): Check Pod annotations
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IP pools from Pod annotation '%s'", constant.AnnoPodIPPools)

	var annoPodIPPools types.AnnoPodIPPoolsValue
	err := json.Unmarshal([]byte(anno), &annoPodIPPools)
	if err != nil {
		return nil, err
	}

	var validIface bool
	for _, v := range annoPodIPPools {
		if v.NIC == nic {
			validIface = true
			break
		}
	}

	if !validIface {
		return nil, fmt.Errorf("the interface of the pod annotation does not contain that requested by runtime: %w", constant.ErrWrongInput)
	}

	var tt []ToBeAllocated
	for _, v := range annoPodIPPools {
		tt = append(tt, ToBeAllocated{
			NIC:              v.NIC,
			V4PoolCandidates: v.IPv4Pools,
			V6PoolCandidates: v.IPv6Pools,
		})
	}

	return tt, nil
}

func (i *ipam) getPoolFromPodAnnoPool(ctx context.Context, anno, nic string) (*ToBeAllocated, error) {
	// TODO(iiiceoo): Check Pod annotations
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IP pools from Pod annotation '%s'", constant.AnnoPodIPPool)

	var annoPodIPPool types.AnnoPodIPPoolValue
	if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
		return nil, err
	}

	if annoPodIPPool.NIC != nil && *annoPodIPPool.NIC != nic {
		return nil, fmt.Errorf("the interface of pod annotation is different from that requested by runtime: %w", constant.ErrWrongInput)
	}

	return &ToBeAllocated{
		NIC:              nic,
		V4PoolCandidates: annoPodIPPool.IPv4Pools,
		V6PoolCandidates: annoPodIPPool.IPv6Pools,
	}, nil
}

func (i *ipam) getPoolFromNS(ctx context.Context, namespace, nic string) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	nsDefautlV4Pools, nsDefautlV6Pools, err := i.nsManager.GetNSDefaultPools(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var t *ToBeAllocated
	if len(nsDefautlV4Pools) != 0 || len(nsDefautlV6Pools) != 0 {
		logger.Sugar().Infof("Use IP pools from Namespace annotation '%s'", constant.AnnotationPre+"/defaultv(4/6)ippool")
		t = &ToBeAllocated{
			NIC:              nic,
			V4PoolCandidates: nsDefautlV4Pools,
			V6PoolCandidates: nsDefautlV6Pools,
		}
	}

	return t, nil
}

func (i *ipam) getPoolFromNetConf(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string) *ToBeAllocated {
	logger := logutils.FromContext(ctx)

	var t *ToBeAllocated
	if len(netConfV4Pool) != 0 || len(netConfV6Pool) != 0 {
		logger.Info("Use IP pools from CNI network configuration")
		t = &ToBeAllocated{
			NIC:              nic,
			V4PoolCandidates: netConfV4Pool,
			V6PoolCandidates: netConfV6Pool,
		}
	}

	return t
}

func (i *ipam) getClusterDefaultPool(ctx context.Context, nic string) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	if len(i.ipamConfig.ClusterDefaultIPv4IPPool) == 0 && len(i.ipamConfig.ClusterDefaultIPv6IPPool) == 0 {
		return nil, fmt.Errorf("no default IP pool is specified: %w", constant.ErrNoAvailablePool)
	}
	logger.Info("Use IP pools from cluster default pools")

	return &ToBeAllocated{
		NIC:              nic,
		V4PoolCandidates: i.ipamConfig.ClusterDefaultIPv4IPPool,
		V6PoolCandidates: i.ipamConfig.ClusterDefaultIPv6IPPool,
	}, nil
}

func (i *ipam) checkIPVersionEnable(ctx context.Context, tt []ToBeAllocated) error {
	logger := logutils.FromContext(ctx)

	if i.ipamConfig.EnableIPv4 && !i.ipamConfig.EnableIPv6 {
		logger.Sugar().Infof("IPv4 network")
	}
	if !i.ipamConfig.EnableIPv4 && i.ipamConfig.EnableIPv6 {
		logger.Sugar().Infof("IPv6 network")
	}
	if i.ipamConfig.EnableIPv4 && i.ipamConfig.EnableIPv6 {
		logger.Sugar().Infof("Dual stack network")
	}

	var errs []error
	for _, t := range tt {
		if err := i.checkPoolMisspecified(ctx, t); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w", utilerrors.NewAggregate(errs))
	}

	return nil
}

func (i *ipam) checkPoolMisspecified(ctx context.Context, t ToBeAllocated) error {
	v4PoolCount := len(t.V4PoolCandidates)
	v6PoolCount := len(t.V6PoolCandidates)

	if i.ipamConfig.EnableIPv4 && v4PoolCount == 0 {
		return fmt.Errorf("%w in interface %s, IPv4 pool is not specified when IPv4 is enabled", constant.ErrWrongInput, t.NIC)
	}
	if i.ipamConfig.EnableIPv6 && v6PoolCount == 0 {
		return fmt.Errorf("%w in interface %s, IPv6 pool is not specified when IPv6 is enabled", constant.ErrWrongInput, t.NIC)
	}
	if !i.ipamConfig.EnableIPv4 && v4PoolCount != 0 {
		return fmt.Errorf("%w in interface %s, IPv4 pool is specified when IPv4 is disabled", constant.ErrWrongInput, t.NIC)
	}
	if !i.ipamConfig.EnableIPv6 && v6PoolCount != 0 {
		return fmt.Errorf("%w in interface %s, IPv6 pool is specified when IPv6 is disabled", constant.ErrWrongInput, t.NIC)
	}

	return nil
}

func (i *ipam) filterPoolCandidates(ctx context.Context, tt []ToBeAllocated, pod *corev1.Pod) ([]ToBeAllocated, error) {
	var filtered []ToBeAllocated
	for _, t := range tt {
		var selectedV4Pools []string
		for _, pool := range t.V4PoolCandidates {
			eligible, err := i.ipPoolManager.SelectByPod(ctx, constant.IPv4, pool, pod)
			if err != nil {
				return nil, err
			}
			if eligible {
				selectedV4Pools = append(selectedV4Pools, pool)
			}
		}
		if i.ipamConfig.EnableIPv4 && len(selectedV4Pools) == 0 {
			return nil, fmt.Errorf("all IPv4 IP pools filtered out: %w", constant.ErrNoAvailablePool)
		}

		var selectedV6Pools []string
		for _, pool := range t.V6PoolCandidates {
			eligible, err := i.ipPoolManager.SelectByPod(ctx, constant.IPv6, pool, pod)
			if err != nil {
				return nil, err
			}
			if eligible {
				selectedV6Pools = append(selectedV6Pools, pool)
			}
		}
		if i.ipamConfig.EnableIPv6 && len(selectedV6Pools) == 0 {
			return nil, fmt.Errorf("all IPv6 IP pools filtered out: %w", constant.ErrNoAvailablePool)
		}

		filtered = append(filtered, ToBeAllocated{
			NIC:              t.NIC,
			V4PoolCandidates: selectedV4Pools,
			V6PoolCandidates: selectedV6Pools,
		})
	}

	return filtered, nil
}

func (i *ipam) verifyPoolCandidates(ctx context.Context, tt []ToBeAllocated) error {
	for _, t := range tt {
		allPools := append(t.V4PoolCandidates, t.V6PoolCandidates...)
		vlanToPools, same, err := i.ipPoolManager.CheckVlanSame(ctx, allPools)
		if err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("vlans in each IP pools are not same: %w, details: %v", constant.ErrWrongInput, vlanToPools)
		}
	}

	// TODO(iiiceoo): Check CIDR overlap
	return nil
}

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to release IP")

	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *delArgs.PodNamespace, *delArgs.PodName, *delArgs.ContainerID, *delArgs.IfName, true)
	if err != nil {
		return err
	}

	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}

	if !currently {
		logger.Warn("Request to release a non current IP allocation, there may be concurrency between the same pod")
	}

	if err = i.release(ctx, allocation.ContainerID, allocation.IPs); err != nil {
		return err
	}

	if err := i.weManager.ClearCurrentIPAllocation(ctx, *delArgs.PodNamespace, *delArgs.PodName, *delArgs.ContainerID); err != nil {
		return err
	}

	logger.Sugar().Infof("Succeed to release: %+v", allocation.IPs)

	return nil
}

func (i *ipam) release(ctx context.Context, containerID string, details []spiderpoolv1.IPAllocationDetail) error {
	logger := logutils.FromContext(ctx)

	if len(details) == 0 {
		return nil
	}

	poolToIPAndCIDs := groupIPDetails(containerID, details)
	errCh := make(chan error, len(poolToIPAndCIDs))
	wg := sync.WaitGroup{}
	wg.Add(len(poolToIPAndCIDs))

	for pool, ipAndCIDs := range poolToIPAndCIDs {
		go func(pool string, ipAndCIDs []ippoolmanager.IPAndCID) {
			defer wg.Done()

			_, err := i.ipamLimiter.AcquireTicket(ctx, pool)
			if err != nil {
				logger.Sugar().Errorf("Failed to queue correctly: %v", err)
			} else {
				defer i.ipamLimiter.ReleaseTicket(ctx, pool)
			}

			if err := i.ipPoolManager.ReleaseIP(ctx, pool, ipAndCIDs); err != nil {
				errCh <- err
				return
			}
			logger.Sugar().Infof("Succeed to release IP %+v from IP pool %s", ipAndCIDs, pool)
		}(pool, ipAndCIDs)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to release all allocated IP %+v: %w", poolToIPAndCIDs, utilerrors.NewAggregate(errs))
	}

	return nil
}

func (i *ipam) Start(ctx context.Context) error {
	return i.ipamLimiter.Start(ctx)
}
