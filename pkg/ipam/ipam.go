// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"errors"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
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
	if stsManager == nil {
		return nil, errors.New("StatefulSetManager must be specified")
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
		return nil, fmt.Errorf("failed to get Pod: %v", err)
	}
	podStatus, allocatable := podmanager.CheckPodStatus(pod)
	if !allocatable {
		return nil, fmt.Errorf("%w: %s Pod", constant.ErrNotAllocatablePod, podStatus)
	}

	endpoint, err := i.weManager.GetEndpointByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get Endpoint: %v", err)
	}

	if i.ipamConfig.EnabledStatefulSet && podmanager.GetControllerOwnerType(pod) == constant.OwnerStatefulSet {
		addResp, err := i.retrieveStsIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, pod, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation of StatefulSet: %v", err)
		}
		if addResp != nil {
			return addResp, nil
		}
	} else {
		addResp, err := i.retrieveMultiNICIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation in multi-NIC mode: %v", err)
		}
		if addResp != nil {
			return addResp, nil
		}
	}

	return i.allocateInStandardMode(ctx, addArgs, pod, endpoint)
}

func (i *ipam) retrieveStsIPAllocation(ctx context.Context, containerID, nic string, pod *corev1.Pod, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	if endpoint == nil {
		logger.Debug("Endpoint not found, do not retrieve anything for StatefulSet and try to allocate IP in standard mode")
		return nil, nil
	}

	// There's no possible that a StatefulSet Pod Endpoint's field 'Status.Current' is nil.
	if endpoint.Status.Current == nil {
		return nil, fmt.Errorf("current IP allocation is lost, endpoint data broken: %+v", endpoint)
	}

	logger.Info("Retrieve the IP allocation of StatefulSet")
	nicMatched := false
	ips, routes := convertIPDetailsToIPConfigsAndAllRoutes(endpoint.Status.Current.IPs)
	for _, c := range ips {
		if *c.Nic == nic {
			if err := i.ipPoolManager.UpdateAllocatedIPs(ctx, containerID, pod, *c); err != nil {
				return nil, fmt.Errorf("failed to update the IPPool IPs of StatefulSet: %v", err)
			}
			nicMatched = true
		}
	}

	if !nicMatched {
		return nil, fmt.Errorf("nic %s do not match the current IP allocation of StatefulSet: %s", nic, endpoint.Status.Current)
	}

	// Refresh Endpoint current IP allocation.
	if err := i.weManager.UpdateCurrentStatus(ctx, containerID, pod); err != nil {
		return nil, fmt.Errorf("failed to update the current IP allocation of StatefulSet: %v", err)
	}

	addResp := &models.IpamAddResponse{
		Ips:    ips,
		Routes: routes,
	}
	logger.Sugar().Infof("Succeed to retrieve the IP allocation of StatefulSet: %+v", *addResp)

	return addResp, nil
}

func (i *ipam) retrieveMultiNICIPAllocation(ctx context.Context, containerID, nic string, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Debug("Retrieve the existing IP allocation in multi-NIC mode")

	allocation, _ := workloadendpointmanager.RetrieveIPAllocation(containerID, nic, false, endpoint)
	if allocation == nil {
		logger.Debug("Nothing retrieved to allocate")
		return nil, nil
	}

	ips, routes := convertIPDetailsToIPConfigsAndAllRoutes(allocation.IPs)
	addResp := &models.IpamAddResponse{
		Ips:    ips,
		Routes: routes,
	}
	logger.Sugar().Infof("Succeed to retrieve the IP allocation in multi-NIC mode: %+v", *addResp)

	return addResp, nil
}

func (i *ipam) allocateInStandardMode(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Debug("Allocate IP in standard mode")

	toBeAllocatedSet, err := i.genToBeAllocatedSet(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, addArgs.CleanGateway, pod)
	if err != nil {
		return nil, err
	}
	results, endpoint, err := i.allocateForAllNICs(ctx, toBeAllocatedSet, *addArgs.ContainerID, endpoint, pod)
	resIPs, resRoutes := convertResultsToIPConfigsAndAllRoutes(results)
	if err != nil {
		// If there are any other errors that might have been thrown at Allocate
		// after the allocateForAllInterfaces is called, use defer.
		if len(resIPs) != 0 {
			if rollbackErr := i.release(ctx, *addArgs.ContainerID, convertResultsToIPDetails(results)); rollbackErr != nil {
				logger.Sugar().Warnf("Failed to roll back the allocated IPs: %v", rollbackErr)
				return nil, err
			}
		}

		if err := i.weManager.ClearCurrentIPAllocation(ctx, *addArgs.ContainerID, endpoint); err != nil {
			logger.Sugar().Warnf("Failed to clear the current IP allocation: %v", err)
		}

		return nil, err
	}

	addResp := &models.IpamAddResponse{
		Ips:    resIPs,
		Routes: resRoutes,
	}
	logger.Sugar().Infof("Succeed to allocate: %+v", *addResp)

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

func (i *ipam) allocateForAllNICs(ctx context.Context, tt []*ToBeAllocated, containerID string, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	logger := logutils.FromContext(ctx)

	customRoutes, err := getCustomRoutes(ctx, pod)
	if err != nil {
		return nil, nil, err
	}

	// TODO(iiiceoo): Comment why containerID should be written first.
	endpoint, err = i.weManager.MarkIPAllocation(ctx, containerID, endpoint, pod)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to mark IP allocation: %v", err)
	}

	var allResults []*AllocationResult
	for _, t := range tt {
		oneResults, endpoint, err := i.allocateForOneNIC(ctx, t, containerID, &customRoutes, endpoint, pod)
		if len(oneResults) != 0 {
			allResults = append(allResults, oneResults...)
		}
		if err != nil {
			return allResults, endpoint, err
		}
	}
	if len(customRoutes) != 0 {
		logger.Sugar().Warnf("Invalid custom routes: %v", customRoutes)
	}

	ips, _ := convertResultsToIPConfigsAndAllRoutes(allResults)
	anno, err := genIPAssignmentAnnotation(ips)
	if err != nil {
		return allResults, endpoint, err
	}

	if err := i.podManager.MergeAnnotations(ctx, pod.Namespace, pod.Name, anno); err != nil {
		return allResults, endpoint, fmt.Errorf("failed to merge IP assignment annotation of Pod: %v", err)
	}

	return allResults, endpoint, nil
}

func (i *ipam) allocateForOneNIC(ctx context.Context, t *ToBeAllocated, containerID string, customRoutes *[]*models.Route, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, *spiderpoolv1.SpiderEndpoint, error) {
	var results []*AllocationResult
	for _, c := range t.PoolCandidates {
		result, err := i.allocateIPFromPoolCandidates(ctx, c, t.NIC, containerID, t.CleanGateway, pod)
		if result.IP != nil {
			results = append(results, result)
		}
		if err != nil {
			return results, endpoint, err
		}

		routes, err := groupCustomRoutesByGW(ctx, customRoutes, result.IP)
		if err != nil {
			return results, endpoint, err
		}
		result.Routes = append(result.Routes, routes...)

		patch := convertResultsToIPDetails([]*AllocationResult{result})
		endpoint, err = i.weManager.PatchIPAllocation(ctx, &spiderpoolv1.PodIPAllocation{
			ContainerID: containerID,
			IPs:         patch,
		}, endpoint)
		if err != nil {
			return results, endpoint, fmt.Errorf("failed to update IP allocation detail %+v of Endpoint: %v", patch, err)
		}
	}

	return results, endpoint, nil
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
	// subnet manager
	if i.ipamConfig.EnableSubnetManager {
		_, enableSubnetMgrV4 := pod.Annotations[subnetmanager.AnnoSubnetManagerV4]
		_, enableSubnetMgrV6 := pod.Annotations[subnetmanager.AnnoSubnetManagerV6]
		if enableSubnetMgrV4 || enableSubnetMgrV6 {
			t, err := i.getPoolFromSubnet(ctx, podmanager.GetControllerOwnerType(pod), pod.Namespace, podmanager.GetControllerOwnerName(pod), nic, cleanGateway)
			if nil != err {
				return nil, err
			}

			if t == nil {
				return nil, fmt.Errorf("pod '%s/%s' specified to use subnet manager but can not found corresponding IPPool", pod.Namespace, pod.Name)
			}

			return []*ToBeAllocated{t}, nil
		}

		return nil, fmt.Errorf("pod '%s/%s' didn't spicify subnet manager when SubnetManager function is enabled", pod.Namespace, pod.Name)
	}

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
	if err != nil {
		return fmt.Errorf("failed to get Pod: %v", err)
	}

	if i.ipamConfig.EnabledStatefulSet && podmanager.GetControllerOwnerType(pod) == constant.OwnerStatefulSet {
		isValidStsPod, err := i.stsManager.IsValidStatefulSetPod(ctx, pod.Namespace, pod.Name, podmanager.GetControllerOwnerType(pod))
		if err != nil {
			return err
		}
		if isValidStsPod {
			logger.Info("No need to release the IP allocation of an StatefulSet whose scale is not reduced")
			return nil
		}
	}

	endpoint, err := i.weManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get Endpoint: %v", err)
	}
	allocation, currently := workloadendpointmanager.RetrieveIPAllocation(*delArgs.ContainerID, *delArgs.IfName, true, endpoint)
	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}
	if !currently {
		logger.Warn("Request to release non current IP allocation, concurrency may exist between the same Pod")
	}

	if err = i.release(ctx, allocation.ContainerID, allocation.IPs); err != nil {
		return err
	}
	if err := i.weManager.ClearCurrentIPAllocation(ctx, *delArgs.ContainerID, endpoint); err != nil {
		return fmt.Errorf("failed to clear current IP allocation: %v", err)
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

func (i *ipam) getPoolFromSubnet(ctx context.Context, podControllerType, podControllerNS, podControllerName string, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	poolList, err := i.ipPoolManager.ListIPPools(ctx,
		client.MatchingLabels{subnetmanager.OwnedApplication: subnetmanager.AppName(podControllerType, podControllerNS, podControllerName)},
	)
	if nil != err {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	if len(poolList.Items) == 0 {
		return nil, nil
	}

	logger.Info("Use IPPools from subnet manager")
	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}

	for _, pool := range poolList.Items {
		if pool.Spec.IPVersion == nil {
			logger.Sugar().Errorf("IPPool '%v' doesn't specify IP version", pool)
			continue
		}

		if *pool.Spec.IPVersion == constant.IPv4 {
			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv4,
				Pools:     pool.Spec.IPs,
			})
		} else {
			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv6,
				Pools:     pool.Spec.IPs,
			})
		}
	}

	if len(t.PoolCandidates) == 0 {
		return nil, nil
	}

	return t, nil
}
