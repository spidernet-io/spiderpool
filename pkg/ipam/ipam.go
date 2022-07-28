// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
		addResp.Ips, addResp.Routes = convertIPDetailsToIPConfigsAndAllRoutes(allocation.IPs)
		return addResp, nil
	}

	customRoutes, err := getCustomRoutes(ctx, pod)
	if err != nil {
		return nil, err
	}

	preliminary, err := i.getPoolCandidates(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IP pool candidates: %+v", preliminary)

	if err := i.ipamConfig.checkIPVersionEnable(ctx, preliminary); err != nil {
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

	results, err := i.allocateForAllNICs(ctx, toBeAllocatedSet, *addArgs.ContainerID, pod)
	resIPs, resRoutes := convertResultsToIPConfigsAndAllRoutes(results)
	if err != nil {
		// If there are any other errors that might have been thrown at Allocate
		// after the allocateForAllInterfaces is called, use defer.
		if len(resIPs) != 0 {
			if err := i.release(ctx, *addArgs.ContainerID, convertResultsToIPDetails(results)); err != nil {
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
	addResp.Routes = append(resRoutes, customRoutes...)
	logger.Sugar().Infof("Succeed to allocate: %+v", addResp)

	return addResp, nil
}

func (i *ipam) allocateForAllNICs(ctx context.Context, tt []*ToBeAllocated, containerID string, pod *corev1.Pod) ([]*AllocationResult, error) {
	// TODO(iiiceoo): Comment why containerID should be written first.
	if err := i.weManager.MarkIPAllocation(ctx, pod.Spec.NodeName, pod.Namespace, pod.Name, containerID); err != nil {
		return nil, fmt.Errorf("failed to mark IP allocation: %v", err)
	}

	var allResults []*AllocationResult
	for _, t := range tt {
		oneResults, err := i.allocateForOneNIC(ctx, t, containerID, pod)
		if len(oneResults) != 0 {
			allResults = append(allResults, oneResults...)
		}
		if err != nil {
			return allResults, err
		}
	}

	ips, _ := convertResultsToIPConfigsAndAllRoutes(allResults)
	anno, err := genIPAssignmentAnnotation(ips)
	if err != nil {
		return allResults, err
	}

	if err := i.podManager.MergeAnnotations(ctx, pod.Namespace, pod.Name, anno); err != nil {
		return allResults, fmt.Errorf("failed to merge IP assignment annotation of pod: %v", err)
	}

	return allResults, nil
}

func (i *ipam) allocateForOneNIC(ctx context.Context, t *ToBeAllocated, containerID string, pod *corev1.Pod) ([]*AllocationResult, error) {
	var results []*AllocationResult
	if t.IPVersion == constant.IPv4 || t.IPVersion == constant.Dual {
		result, err := i.allocateIPFromPoolCandidates(ctx, constant.IPv4, t.NIC, t.DefaultRouteType, t.V4PoolCandidates, containerID, pod)
		if result.IP != nil {
			results = append(results, result)
		}
		if err != nil {
			return results, err
		}
	}

	if t.IPVersion == constant.IPv6 || t.IPVersion == constant.Dual {
		result, err := i.allocateIPFromPoolCandidates(ctx, constant.IPv6, t.NIC, t.DefaultRouteType, t.V6PoolCandidates, containerID, pod)
		if result.IP != nil {
			results = append(results, result)
		}
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

func (i *ipam) allocateIPFromPoolCandidates(ctx context.Context, version types.IPVersion, nic string, defaultRouteType types.DefaultRouteType, poolCandidates []string, containerID string, pod *corev1.Pod) (*AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	// TODO(iiiceoo): Comment why queue up before allocating IP.
	_, err := i.ipamLimiter.AcquireTicket(ctx, poolCandidates...)
	if err != nil {
		logger.Sugar().Errorf("Failed to queue correctly: %v", err)
	} else {
		defer i.ipamLimiter.ReleaseTicket(ctx, poolCandidates...)
	}

	var errs []error
	result := &AllocationResult{}
	for _, pool := range poolCandidates {
		var err error
		result.IP, err = i.ipPoolManager.AllocateIP(ctx, pool, containerID, nic, pod)
		if err != nil {
			errs = append(errs, err)
			logger.Sugar().Warnf("Failed to allocate IPv%d IP to %s from IP pool %s: %v", version, nic, pool, err)
			continue
		}
		logger.Sugar().Infof("Allocate IPv%d IP %s to %s from IP pool %s", version, *result.IP.Address, nic, pool)

		if defaultRouteType == constant.MultiNICNotDefaultRoute {
			result.IP.Gateway = ""
		}

		// TODO(iiiceoo): Use IPPool YAML
		ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, pool)
		if nil != err {
			errs = append(errs, err)
			continue
		}
		result.Routes = append(result.Routes, convertSpecRoutesToOAIRoutes(ipPool.Spec.Routes)...)
		break
	}

	if len(errs) == len(poolCandidates) {
		return result, fmt.Errorf("failed to allocate any IPv%d IP to %s from IP pools %v: %v", version, nic, poolCandidates, utilerrors.NewAggregate(errs).Error())
	}

	patch := convertResultsToIPDetails([]*AllocationResult{result})
	if err := i.weManager.PatchIPAllocation(ctx, pod.Namespace, pod.Name, &spiderpoolv1.PodIPAllocation{
		ContainerID: containerID,
		IPs:         patch,
	}); err != nil {
		return result, fmt.Errorf("failed to update IP allocation detail %+v of workload endpoint: %v", patch, err)
	}

	return result, nil
}

func (i *ipam) getPoolCandidates(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		return getPoolFromPodAnnoPools(ctx, anno, nic)
	}

	if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		t, err := getPoolFromPodAnnoPool(ctx, anno, nic)
		if err != nil {
			return nil, err
		}
		return []*ToBeAllocated{t}, nil
	}

	t, err := i.getPoolFromNS(ctx, pod.Namespace, nic)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return []*ToBeAllocated{t}, nil
	}

	if t := getPoolFromNetConf(ctx, nic, netConfV4Pool, netConfV6Pool); t != nil {
		return []*ToBeAllocated{t}, nil
	}

	t, err = i.ipamConfig.getClusterDefaultPool(ctx, nic)
	if err != nil {
		return nil, err
	}

	return []*ToBeAllocated{t}, nil
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
			DefaultRouteType: constant.SingleNICDefaultRoute,
			V4PoolCandidates: nsDefautlV4Pools,
			V6PoolCandidates: nsDefautlV6Pools,
		}
	}

	return t, nil
}

func (i *ipam) filterPoolCandidates(ctx context.Context, tt []*ToBeAllocated, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	var filtered []*ToBeAllocated
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
			return nil, fmt.Errorf("%w, all IPv4 IP pools of %s filtered out", constant.ErrNoAvailablePool, t.NIC)
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
			return nil, fmt.Errorf("%w, all IPv6 IP pools of %s filtered out", constant.ErrNoAvailablePool, t.NIC)
		}

		filtered = append(filtered, &ToBeAllocated{
			IPVersion:        t.IPVersion,
			NIC:              t.NIC,
			V4PoolCandidates: selectedV4Pools,
			V6PoolCandidates: selectedV6Pools,
			DefaultRouteType: t.DefaultRouteType,
		})
	}

	return filtered, nil
}

func (i *ipam) verifyPoolCandidates(ctx context.Context, tt []*ToBeAllocated) error {
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
