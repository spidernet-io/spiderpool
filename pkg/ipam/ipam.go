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
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
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
	stsManager    statefulsetmanager.StatefulSetManager
}

func NewIPAM(c *IPAMConfig, ipPoolManager ippoolmanager.IPPoolManager, weManager workloadendpointmanager.WorkloadEndpointManager,
	nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager, stsManager statefulsetmanager.StatefulSetManager) (IPAM, error) {
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
		stsManager:    stsManager,
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

	// StatefulSet
	if i.ipamConfig.EnabledStatefulSet && podmanager.GetControllerOwnerType(pod) == constant.OwnerStatefulSet {
		ipamAddResp, err := i.retrieveStsAllocatedIPs(ctx, *addArgs.ContainerID, pod, we)
		if nil != err {
			return nil, fmt.Errorf("failed to retrieve StatefulSet allocated IPs, error: %v", err)
		}

		if ipamAddResp != nil {
			return ipamAddResp, nil
		}
	}

	addResp := &models.IpamAddResponse{}
	allocation, currently, err := i.weManager.RetrieveIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, false, we)
	if err != nil {
		return nil, err
	}

	if allocation != nil && currently {
		logger.Sugar().Infof("Retrieve an existing IP allocation: %+v", allocation.IPs)
		addResp.Ips, addResp.Routes = convertIPDetailsToIPConfigsAndAllRoutes(allocation.IPs)
		return addResp, nil
	}

	toBeAllocatedSet, err := i.genToBeAllocatedSet(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, addArgs.CleanGateway, pod)
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
	addResp.Routes = resRoutes
	logger.Sugar().Infof("Succeed to allocate: %+v", addResp)

	return addResp, nil
}

func (i *ipam) genToBeAllocatedSet(ctx context.Context, nic string, defaultIPV4IPPool, defaultIPV6IPPool []string, cleanGateway bool, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	preliminary, err := i.getPoolCandidates(ctx, nic, defaultIPV4IPPool, defaultIPV6IPPool, cleanGateway, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IPPool candidates: %s", preliminary)

	if err := i.ipamConfig.checkIPVersionEnable(ctx, preliminary); err != nil {
		return nil, err
	}

	toBeAllocatedSet, err := i.filterPoolCandidates(ctx, preliminary, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Filtered IPPool candidates: %s", toBeAllocatedSet)

	if err := i.verifyPoolCandidates(ctx, toBeAllocatedSet); err != nil {
		return nil, err
	}
	logger.Info("All IPPool candidates valid")

	return toBeAllocatedSet, nil
}

func (i *ipam) allocateForAllNICs(ctx context.Context, tt []*ToBeAllocated, containerID string, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	logger := logutils.FromContext(ctx)

	customRoutes, err := getCustomRoutes(ctx, pod)
	if err != nil {
		return nil, nil, err
	}

	// TODO(iiiceoo): Comment why containerID should be written first.
	we, err = i.weManager.MarkIPAllocation(ctx, containerID, we, pod)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to mark IP allocation: %v", err)
	}

	var allResults []*AllocationResult
	for _, t := range tt {
		oneResults, we, err := i.allocateForOneNIC(ctx, t, containerID, &customRoutes, we, pod)
		if len(oneResults) != 0 {
			allResults = append(allResults, oneResults...)
		}
		if err != nil {
			return allResults, we, err
		}
	}
	if len(customRoutes) != 0 {
		logger.Sugar().Warnf("Invalid custom routes: %v", customRoutes)
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
func (i *ipam) allocateForOneNIC(ctx context.Context, t *ToBeAllocated, containerID string, customRoutes *[]*models.Route, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	var results []*AllocationResult
	for _, c := range t.PoolCandidates {
		result, err := i.allocateIPFromPoolCandidates(ctx, c, t.NIC, containerID, t.CleanGateway, pod)
		if result.IP != nil {
			results = append(results, result)
		}
		if err != nil {
			return results, we, err
		}

		routes, err := groupCustomRoutesByGW(ctx, customRoutes, result.IP)
		if err != nil {
			return results, we, err
		}
		result.Routes = append(result.Routes, routes...)

		patch := convertResultsToIPDetails([]*AllocationResult{result})
		we, err = i.weManager.PatchIPAllocation(ctx, &spiderpoolv1.PodIPAllocation{
			ContainerID: containerID,
			IPs:         patch,
		}, we)
		if err != nil {
			return results, we, fmt.Errorf("failed to update IP allocation detail %+v of Endpoint: %v", patch, err)
		}
	}

	return results, we, nil
}

func (i *ipam) allocateIPFromPoolCandidates(ctx context.Context, c *PoolCandidate, nic, containerID string, cleanGateway bool, pod *corev1.Pod) (*AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	// TODO(iiiceoo): Comment why queue up before allocating IP.
	_, err := i.ipamLimiter.AcquireTicket(ctx, c.Pools...)
	if err != nil {
		logger.Sugar().Errorf("Failed to queue correctly: %v", err)
	} else {
		defer i.ipamLimiter.ReleaseTicket(ctx, c.Pools...)
	}

	var errs []error
	result := &AllocationResult{}
	for _, pool := range c.Pools {
		var err error
		var ipPool *spiderpoolv1.SpiderIPPool
		result.IP, ipPool, err = i.ipPoolManager.AllocateIP(ctx, pool, containerID, nic, pod)
		if err != nil {
			errs = append(errs, err)
			logger.Sugar().Warnf("Failed to allocate IPv%d IP to %s from IPPool %s: %v", c.IPVersion, nic, pool, err)
			continue
		}
		result.CleanGateway = cleanGateway
		result.Routes = append(result.Routes, convertSpecRoutesToOAIRoutes(nic, ipPool.Spec.Routes)...)
		logger.Sugar().Infof("Allocate IPv%d IP %s to %s from IPPool %s", c.IPVersion, *result.IP.Address, nic, pool)
		break
	}

	if len(errs) == len(c.Pools) {
		return result, fmt.Errorf("failed to allocate any IPv%d IP to %s from IPPools %v: %v", c.IPVersion, nic, c.Pools, utilerrors.NewAggregate(errs).Error())
	}

	return result, nil
}

func (i *ipam) getPoolCandidates(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, cleanGateway bool, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	// pod annotation: "ipam.spidernet.io/ippools"
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		return getPoolFromPodAnnoPools(ctx, anno, nic)
	}

	// pod annotation: "ipam.spidernet.io/ippool"
	if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		t, err := getPoolFromPodAnnoPool(ctx, anno, nic, cleanGateway)
		if err != nil {
			return nil, err
		}
		return []*ToBeAllocated{t}, nil
	}

	// namespace annotation: "ipam.spidernet.io/defaultv4ippool" and "ipam.spidernet.io/defaultv6ippool"
	t, err := i.getPoolFromNS(ctx, pod.Namespace, nic, cleanGateway)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return []*ToBeAllocated{t}, nil
	}

	// IPAM configuration file
	if t := getPoolFromNetConf(ctx, nic, netConfV4Pool, netConfV6Pool, cleanGateway); t != nil {
		return []*ToBeAllocated{t}, nil
	}

	// configmap
	t, err = i.ipamConfig.getClusterDefaultPool(ctx, nic, cleanGateway)
	if err != nil {
		return nil, err
	}

	return []*ToBeAllocated{t}, nil
}

func (i *ipam) getPoolFromNS(ctx context.Context, namespace, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	ns, err := i.nsManager.GetNamespaceByName(ctx, namespace)
	if err != nil {
		return nil, err
	}
	nsDefaultV4Pools, nsDefaultV6Pools, err := i.nsManager.GetNSDefaultPools(ctx, ns)
	if err != nil {
		return nil, err
	}

	if len(nsDefaultV4Pools) == 0 && len(nsDefaultV6Pools) == 0 {
		return nil, nil
	}

	logger.Sugar().Infof("Use IPPools from Namespace annotation '%s'", constant.AnnotationPre+"/defaultv(4/6)ippool")
	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}
	if len(nsDefaultV4Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     nsDefaultV4Pools,
		})
	}
	if len(nsDefaultV6Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     nsDefaultV6Pools,
		})
	}

	return t, nil
}

func (i *ipam) filterPoolCandidates(ctx context.Context, tt []*ToBeAllocated, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	for _, t := range tt {
		for _, c := range t.PoolCandidates {
			var selectedPools []string
			for _, pool := range c.Pools {
				eligible, err := i.ipPoolManager.SelectByPod(ctx, c.IPVersion, pool, pod)
				if err != nil {
					return nil, err
				}
				if eligible {
					selectedPools = append(selectedPools, pool)
				}
			}
			if len(selectedPools) == 0 {
				return nil, fmt.Errorf("%w, all IPv%d IPPools of %s filtered out", constant.ErrNoAvailablePool, c.IPVersion, t.NIC)
			}
			c.Pools = selectedPools
		}
	}

	return tt, nil
}

func (i *ipam) verifyPoolCandidates(ctx context.Context, tt []*ToBeAllocated) error {
	for _, t := range tt {
		var allPools []string
		for _, c := range t.PoolCandidates {
			allPools = append(allPools, c.Pools...)
		}
		vlanToPools, same, err := i.ipPoolManager.CheckVlanSame(ctx, allPools)
		if err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("%w, vlans in each IPPools are not same: %v", constant.ErrWrongInput, vlanToPools)
		}
	}

	return nil
}

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to release IP")

	pod, err := i.podManager.GetPodByName(ctx, *delArgs.PodNamespace, *delArgs.PodName)
	if nil != err {
		return err
	}

	// StatefulSet
	if i.ipamConfig.EnabledStatefulSet && podmanager.GetControllerOwnerType(pod) == constant.OwnerStatefulSet {
		isValidStsPod, err := i.stsManager.IsValidStatefulSetPod(ctx, pod.Namespace, pod.Name, podmanager.GetControllerOwnerType(pod))
		if nil != err {
			return fmt.Errorf("failed to check whether clean up StatefulSet pod, error: %v", err)
		}

		if isValidStsPod {
			logger.Sugar().Infof("no need to release for StatefulSet pod '%s/%s'", pod.Namespace, pod.Name)
			return nil
		}
	}

	we, err := i.weManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	allocation, currently, err := i.weManager.RetrieveIPAllocation(ctx, *delArgs.ContainerID, *delArgs.IfName, true, we)
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

	poolToIPAndCIDs := GroupIPDetails(containerID, details)
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
