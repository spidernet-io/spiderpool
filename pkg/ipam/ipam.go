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
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		return nil, errors.New("IPAM config must be specified")
	}
	if ipPoolManager == nil {
		return nil, errors.New("IPPoolManager must be specified")
	}
	if weManager == nil {
		return nil, errors.New("EndpointManager must be specified")
	}
	if nsManager == nil {
		return nil, errors.New("NamespaceManager must be specified")
	}
	if podManager == nil {
		return nil, errors.New("PodManager must be specified")
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
		return nil, fmt.Errorf("failed to get Pod %s: %v", *addArgs.PodName, err)
	}
	podStatus, allocatable := i.podManager.CheckPodStatus(pod)
	if !allocatable {
		return nil, fmt.Errorf("%w: %s Pod", constant.ErrNotAllocatablePod, podStatus)
	}

	we, err := i.weManager.GetEndpointByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, false, we)
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

	toBeAllocatedSet, err := i.genToBeAllocatedSet(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, pod)
	if err != nil {
		return nil, err
	}
	results, we, err := i.allocateForAllNICs(ctx, toBeAllocatedSet, *addArgs.ContainerID, we, pod)
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

		if err := i.weManager.ClearCurrentIPAllocation(ctx, *addArgs.ContainerID, we); err != nil {
			logger.Sugar().Warnf("Failed to clear current IP allocation: %v", err)
		}

		return nil, err
	}

	addResp.Ips = resIPs
	addResp.Routes = append(resRoutes, customRoutes...)
	logger.Sugar().Infof("Succeed to allocate: %+v", addResp)

	return addResp, nil
}

func (i *ipam) genToBeAllocatedSet(ctx context.Context, nic string, defaultIPV4IPPool, defaultIPV6IPPool []string, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	preliminary, err := i.getPoolCandidates(ctx, nic, defaultIPV4IPPool, defaultIPV6IPPool, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IPPool candidates: %+v", preliminary)

	if err := i.ipamConfig.checkIPVersionEnable(ctx, preliminary); err != nil {
		return nil, err
	}

	toBeAllocatedSet, err := i.filterPoolCandidates(ctx, preliminary, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Filtered IPPool candidates: %+v", toBeAllocatedSet)

	if err := i.verifyPoolCandidates(ctx, toBeAllocatedSet); err != nil {
		return nil, err
	}
	logger.Info("All IPPool candidates valid")

	return toBeAllocatedSet, nil
}

func (i *ipam) allocateForAllNICs(ctx context.Context, tt []*ToBeAllocated, containerID string, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	// TODO(iiiceoo): Comment why containerID should be written first.
	we, err := i.weManager.MarkIPAllocation(ctx, containerID, we, pod)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to mark IP allocation: %v", err)
	}

	var allResults []*AllocationResult
	for _, t := range tt {
		oneResults, we, err := i.allocateForOneNIC(ctx, t, containerID, we, pod)
		if len(oneResults) != 0 {
			allResults = append(allResults, oneResults...)
		}
		if err != nil {
			return allResults, we, err
		}
	}

	ips, _ := convertResultsToIPConfigsAndAllRoutes(allResults)
	anno, err := genIPAssignmentAnnotation(ips)
	if err != nil {
		return allResults, we, err
	}

	if err := i.podManager.MergeAnnotations(ctx, pod.Namespace, pod.Name, anno); err != nil {
		return allResults, we, fmt.Errorf("failed to merge IP assignment annotation of Pod: %v", err)
	}

	return allResults, we, nil
}

func (i *ipam) allocateForOneNIC(ctx context.Context, t *ToBeAllocated, containerID string, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	var results []*AllocationResult
	if t.IPVersion == constant.IPv4 || t.IPVersion == constant.Dual {
		result, we, err := i.allocateIPFromPoolCandidates(ctx, constant.IPv4, t, containerID, we, pod)
		if result.IP != nil {
			results = append(results, result)
		}
		if err != nil {
			return results, we, err
		}
	}

	if t.IPVersion == constant.IPv6 || t.IPVersion == constant.Dual {
		result, we, err := i.allocateIPFromPoolCandidates(ctx, constant.IPv6, t, containerID, we, pod)
		if result.IP != nil {
			results = append(results, result)
		}
		if err != nil {
			return results, we, err
		}
	}

	return results, we, nil
}

func (i *ipam) allocateIPFromPoolCandidates(ctx context.Context, version types.IPVersion, t *ToBeAllocated, containerID string, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) (*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	logger := logutils.FromContext(ctx)

	// TODO(iiiceoo): Refactor
	var poolCandidates []string
	if version == constant.IPv4 {
		poolCandidates = t.V4PoolCandidates
	} else if version == constant.IPv6 {
		poolCandidates = t.V6PoolCandidates
	}

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
		var ipPool *spiderpoolv1.SpiderIPPool
		result.IP, ipPool, err = i.ipPoolManager.AllocateIP(ctx, pool, containerID, t.NIC, pod)
		if err != nil {
			errs = append(errs, err)
			logger.Sugar().Warnf("Failed to allocate IPv%d IP to %s from IPPool %s: %v", version, t.NIC, pool, err)
			continue
		}
		logger.Sugar().Infof("Allocate IPv%d IP %s to %s from IPPool %s", version, *result.IP.Address, t.NIC, pool)

		if t.DefaultRouteType == constant.MultiNICNotDefaultRoute {
			result.IP.Gateway = ""
		}
		result.Routes = append(result.Routes, convertSpecRoutesToOAIRoutes(ipPool.Spec.Routes)...)
		break
	}

	if len(errs) == len(poolCandidates) {
		return result, we, fmt.Errorf("failed to allocate any IPv%d IP to %s from IPPools %v: %v", version, t.NIC, poolCandidates, utilerrors.NewAggregate(errs).Error())
	}

	patch := convertResultsToIPDetails([]*AllocationResult{result})
	we, err = i.weManager.PatchIPAllocation(ctx, &spiderpoolv1.PodIPAllocation{
		ContainerID: containerID,
		IPs:         patch,
	}, we)
	if err != nil {
		return result, we, fmt.Errorf("failed to update IP allocation detail %+v of Endpoint: %v", patch, err)
	}

	return result, we, nil
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

	ns, err := i.nsManager.GetNamespaceByName(ctx, namespace)
	if err != nil {
		return nil, err
	}
	nsDefautlV4Pools, nsDefautlV6Pools, err := i.nsManager.GetNSDefaultPools(ctx, ns)
	if err != nil {
		return nil, err
	}

	var t *ToBeAllocated
	if len(nsDefautlV4Pools) != 0 || len(nsDefautlV6Pools) != 0 {
		logger.Sugar().Infof("Use IPPools from Namespace annotation '%s'", constant.AnnotationPre+"/defaultv(4/6)ippool")
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
			return nil, fmt.Errorf("%w, all IPv4 IPPools of %s filtered out", constant.ErrNoAvailablePool, t.NIC)
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
			return nil, fmt.Errorf("%w, all IPv6 IPPools of %s filtered out", constant.ErrNoAvailablePool, t.NIC)
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
			return fmt.Errorf("vlans in each IPPools are not same: %w, details: %v", constant.ErrWrongInput, vlanToPools)
		}
	}

	return nil
}

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to release IP")

	we, err := i.weManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *delArgs.ContainerID, *delArgs.IfName, true, we)
	if err != nil {
		return err
	}
	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}
	if !currently {
		logger.Warn("Request to release a non current IP allocation, there may be concurrency between the same Pod")
	}

	if err = i.release(ctx, allocation.ContainerID, allocation.IPs); err != nil {
		return err
	}
	if err := i.weManager.ClearCurrentIPAllocation(ctx, *delArgs.ContainerID, we); err != nil {
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
			logger.Sugar().Infof("Succeed to release IP %+v from IPPool %s", ipAndCIDs, pool)
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
