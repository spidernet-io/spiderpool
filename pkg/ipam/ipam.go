// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

type IPAM interface {
	Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error)
	Release(ctx context.Context, delArgs *models.IpamDelArgs) error
	Start(ctx context.Context) error
}

type ipam struct {
	config        *IPAMConfig
	ipamLimiter   limiter.Limiter
	ipPoolManager ippoolmanagertypes.IPPoolManager
	weManager     workloadendpointmanager.WorkloadEndpointManager
	nodeManager   nodemanager.NodeManager
	nsManager     namespacemanager.NamespaceManager
	podManager    podmanager.PodManager
	stsManager    statefulsetmanager.StatefulSetManager
	subnetManager subnetmanagertypes.SubnetManager
}

func NewIPAM(c *IPAMConfig, ipPoolManager ippoolmanagertypes.IPPoolManager, weManager workloadendpointmanager.WorkloadEndpointManager, nodeManager nodemanager.NodeManager,
	nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager, stsManager statefulsetmanager.StatefulSetManager, subnetMgr subnetmanagertypes.SubnetManager) (IPAM, error) {
	if c == nil {
		return nil, errors.New("ipam config must be specified")
	}
	if ipPoolManager == nil {
		return nil, errors.New("ippool manager must be specified")
	}
	if weManager == nil {
		return nil, errors.New("endpoint manager must be specified")
	}
	if nodeManager == nil {
		return nil, errors.New("node manager must be specified")
	}
	if nsManager == nil {
		return nil, errors.New("namespace manager must be specified")
	}
	if podManager == nil {
		return nil, errors.New("pod manager must be specified")
	}
	if stsManager == nil {
		return nil, errors.New("statefulset manager must be specified")
	}
	if c.EnableSpiderSubnet && subnetMgr == nil {
		return nil, errors.New("subnet manager must be specified")
	}

	ipamLimiter := limiter.NewLimiter(c.LimiterConfig)
	return &ipam{
		config:        c,
		ipamLimiter:   ipamLimiter,
		ipPoolManager: ipPoolManager,
		weManager:     weManager,
		nodeManager:   nodeManager,
		nsManager:     nsManager,
		podManager:    podManager,
		stsManager:    stsManager,
		subnetManager: subnetMgr,
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

	ownerControllerType, _ := podmanager.GetOwnerControllerType(pod)
	if i.config.EnableStatefulSet && ownerControllerType == constant.OwnerStatefulSet {
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

	if err := i.config.checkIPVersionEnable(ctx, preliminary); err != nil {
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
	subnetMgrV4Name, enableSubnetMgrV4 := pod.Annotations[constant.AnnoSubnetManagerV4]
	subnetMgrV6Name, enableSubnetMgrV6 := pod.Annotations[constant.AnnoSubnetManagerV6]
	if enableSubnetMgrV4 || enableSubnetMgrV6 {
		if i.config.EnableSpiderSubnet {
			t, err := i.getPoolFromSubnet(ctx, pod, nic, cleanGateway, subnetMgrV4Name, subnetMgrV6Name)
			if nil != err {
				return nil, err
			}

			return []*ToBeAllocated{t}, nil
		} else {
			return nil, fmt.Errorf("feature SpiderSubnet is disabled, but the Pod annotation for automatically creating the IPPool is specified")
		}
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
	t, err = i.config.getClusterDefaultPool(ctx, nic, cleanGateway)
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
				eligible, err := i.selectByPod(ctx, c.IPVersion, pool, pod)
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

func (i *ipam) selectByPod(ctx context.Context, version types.IPVersion, poolName string, pod *corev1.Pod) (bool, error) {
	logger := logutils.FromContext(ctx)

	ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, poolName)
	if err != nil {
		logger.Sugar().Warnf("Failed to get IPPool %s: %v", poolName, err)
		return false, err
	}

	if ipPool.DeletionTimestamp != nil {
		logger.Sugar().Warnf("IPPool %s is terminating", poolName)
		return false, nil
	}

	if *ipPool.Spec.Disable {
		logger.Sugar().Warnf("IPPool %s is disable", poolName)
		return false, nil
	}

	if *ipPool.Spec.IPVersion != version {
		logger.Sugar().Warnf("IPPool %s has different version with specified via input", poolName)
		return false, nil
	}

	// TODO(iiiceoo): Check whether there are any unused IP

	if ipPool.Spec.NodeAffinity != nil {
		nodeMatched, err := i.nodeManager.MatchLabelSelector(ctx, pod.Spec.NodeName, ipPool.Spec.NodeAffinity)
		if err != nil {
			return false, err
		}
		if !nodeMatched {
			logger.Sugar().Infof("Unmatched Node selector, IPPool %s is filtered", poolName)
			return false, nil
		}
	}

	if ipPool.Spec.NamespaceAffinity != nil {
		nsMatched, err := i.nsManager.MatchLabelSelector(ctx, pod.Namespace, ipPool.Spec.NamespaceAffinity)
		if err != nil {
			return false, err
		}
		if !nsMatched {
			logger.Sugar().Infof("Unmatched Namespace selector, IPPool %s is filtered", poolName)
			return false, nil
		}
	}

	if ipPool.Spec.PodAffinity != nil {
		podMatched, err := i.podManager.MatchLabelSelector(ctx, pod.Namespace, pod.Name, ipPool.Spec.PodAffinity)
		if err != nil {
			return false, err
		}
		if !podMatched {
			logger.Sugar().Infof("Unmatched Pod selector, IPPool %s is filtered", poolName)
			return false, nil
		}
	}

	return true, nil
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

	ownerControllerType, _ := podmanager.GetOwnerControllerType(pod)
	if i.config.EnableStatefulSet && ownerControllerType == constant.OwnerStatefulSet {
		isValidStsPod, err := i.stsManager.IsValidStatefulSetPod(ctx, pod.Namespace, pod.Name, ownerControllerType)
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
		go func(pool string, ipAndCIDs []types.IPAndCID) {
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

func (i *ipam) getPoolFromSubnet(ctx context.Context, pod *corev1.Pod, nic string, cleanGateway bool, subnetMgrV4Name, subnetMgrV6Name string) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)
	logger.Info("Use IPPools from subnet manager")

	podTopControllerKind, podTopController, err := i.podManager.GetPodTopController(ctx, pod)
	if nil != err {
		return nil, fmt.Errorf("failed to get pod top owner reference, error: %v", err)
	}
	if podTopControllerKind == constant.OwnerNone || podTopControllerKind == constant.OwnerUnknown {
		return nil, fmt.Errorf("subnet manager doesn't support pod '%s/%s' owner controller", pod.Namespace, pod.Name)
	}

	var ErrPoolNotFound = fmt.Errorf("failed to retrieve IPPools")

	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}

	for j := 0; j <= i.config.WaitSubnetPoolRetries; j++ {
		if len(subnetMgrV4Name) != 0 {
			v4Pool, err := i.subnetManager.RetrieveIPPool(ctx, podTopControllerKind, podTopController, subnetMgrV4Name, constant.IPv4)
			if nil != err {
				if j == i.config.WaitSubnetPoolRetries {
					return nil, fmt.Errorf("%w: %v", ErrPoolNotFound, err)
				}

				logger.Error(err.Error())
				continue
			}

			if v4Pool == nil {
				logger.Sugar().Errorf("failed to retrieve SpiderSubnet '%s' IPPool, error: %v", subnetMgrV4Name)
				time.Sleep(i.config.WaitSubnetPoolTime)
				continue
			}

			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv4,
				Pools:     []string{v4Pool.Name},
			})
		}

		if len(subnetMgrV6Name) != 0 {
			v6Pool, err := i.subnetManager.RetrieveIPPool(ctx, podTopControllerKind, podTopController, subnetMgrV6Name, constant.IPv6)
			if nil != err {
				if j == i.config.WaitSubnetPoolRetries {
					return nil, fmt.Errorf("%w: %v", ErrPoolNotFound, err)
				}

				logger.Error(err.Error())
				continue
			}

			if v6Pool == nil {
				logger.Sugar().Errorf("failed to retrieve SpiderSubnet '%s' IPPool, error: %v", subnetMgrV4Name)
				time.Sleep(i.config.WaitSubnetPoolTime)
				continue
			}

			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv6,
				Pools:     []string{v6Pool.Name},
			})
		}

		break
	}

	return t, nil
}
