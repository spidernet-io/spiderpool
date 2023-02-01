// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/singletons"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	subnetmanagercontrollers "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
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

	return &ipam{
		config:        c,
		ipamLimiter:   limiter.NewLimiter(c.LimiterConfig),
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
	logger.Info("Start to allocate")

	pod, err := i.podManager.GetPodByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Pod %s/%s: %v", *addArgs.PodNamespace, *addArgs.PodName, err)
	}
	podStatus, allocatable := podmanager.CheckPodStatus(pod)
	if !allocatable {
		return nil, fmt.Errorf("%s Pod %s/%s cannot allocate IP addresees", strings.ToLower(string(podStatus)), *addArgs.PodNamespace, *addArgs.PodName)
	}

	endpoint, err := i.weManager.GetEndpointByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get Endpoint %s/%s: %v", *addArgs.PodNamespace, *addArgs.PodName, err)
	}

	podTopController, err := i.podManager.GetPodTopController(ctx, pod)
	if nil != err {
		return nil, err
	}

	if i.config.EnableStatefulSet && podTopController.Kind == constant.KindStatefulSet {
		addResp, err := i.retrieveStsIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, pod, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation of StatefulSet %s/%s: %w", podTopController.Namespace, podTopController.Name, err)
		}
		if addResp != nil {
			return addResp, nil
		}
	} else {
		addResp, err := i.retrieveMultiNICIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation in multi-NIC mode: %w", err)
		}
		if addResp != nil {
			return addResp, nil
		}
	}

	addResp, err := i.allocateInStandardMode(ctx, addArgs, pod, endpoint, podTopController)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP addresses in standard mode: %w", err)
	}

	return addResp, nil
}

func (i *ipam) retrieveStsIPAllocation(ctx context.Context, containerID, nic string, pod *corev1.Pod, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	if endpoint == nil {
		logger.Debug("Endpoint not found, do not retrieve anything for StatefulSet and try to allocate IP in standard mode")
		return nil, nil
	}

	// There's no possible that a StatefulSet Pod Endpoint's field 'Status.Current' is nil.
	if endpoint.Status.Current == nil {
		logger.Sugar().Warnf("SpiderEndpoint '%s/%s' doesn't have current IP allocation, try to re-allocate", endpoint.Namespace, endpoint.Name)
		return nil, nil
	}

	logger.Info("Retrieve the IP allocation of StatefulSet")

	// validation
	for _, allocation := range endpoint.Status.Current.IPs {
		if i.config.EnableIPv4 && allocation.IPv4 == nil || i.config.EnableIPv6 && allocation.IPv6 == nil {
			return nil, fmt.Errorf("StatefulSet pod has legacy failure allocation %v", allocation)
		}
	}

	nicMatched := false
	ips, routes := convertIPDetailsToIPConfigsAndAllRoutes(endpoint.Status.Current.IPs)
	for _, c := range ips {
		if *c.Nic == nic {
			if err := i.ipPoolManager.UpdateAllocatedIPs(ctx, containerID, pod, *c); err != nil {
				return nil, fmt.Errorf("failed to update StatefulSet's previous IP allocation records in IPPool %s: %w", c.IPPool, err)
			}
			nicMatched = true
		}
	}

	if !nicMatched {
		return nil, fmt.Errorf("nic %s do not match the current IP allocation of StatefulSet", nic)
	}

	// Refresh Endpoint current IP allocation.
	if err := i.weManager.ReallocateCurrentIPAllocation(ctx, containerID, pod.Spec.NodeName, pod.Namespace, pod.Name); err != nil {
		return nil, fmt.Errorf("failed to update the current IP allocation of StatefulSet: %w", err)
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
	logger.Debug("Try to retrieve the existing IP allocation in multi-NIC mode")

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

func (i *ipam) allocateInStandardMode(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, endpoint *spiderpoolv1.SpiderEndpoint, podController types.PodTopController) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Info("Allocate IP addresses in standard mode")

	toBeAllocatedSet, err := i.genToBeAllocatedSet(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, addArgs.CleanGateway, pod, podController)
	if err != nil {
		return nil, err
	}

	// TODO(iiiceoo): Comment why containerID should be written first.
	if endpoint == nil {
		endpoint, err = i.weManager.MarkIPAllocation(ctx, *addArgs.ContainerID, pod)
		if err != nil {
			return nil, fmt.Errorf("failed to mark IP allocation: %v", err)
		}
	} else {
		if err := i.weManager.ReMarkIPAllocation(ctx, *addArgs.ContainerID, pod, endpoint); err != nil {
			return nil, fmt.Errorf("failed to remark IP allocation: %v", err)
		}
	}

	results, err := i.allocateForAllNICs(ctx, toBeAllocatedSet, *addArgs.ContainerID, endpoint, pod)
	resIPs, resRoutes := convertResultsToIPConfigsAndAllRoutes(results)
	if err != nil {
		// If there are any other errors that might have been thrown at Allocate
		// after the allocateForAllInterfaces is called, use defer.
		if len(resIPs) != 0 {
			if rollbackErr := i.release(ctx, *addArgs.ContainerID, convertResultsToIPDetails(results)); rollbackErr != nil {
				metric.IpamAllocationRollbackFailureCounts.Add(ctx, 1)
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

func (i *ipam) genToBeAllocatedSet(ctx context.Context, nic string, defaultIPV4IPPool, defaultIPV6IPPool []string, cleanGateway bool, pod *corev1.Pod, podController types.PodTopController) ([]*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	preliminary, err := i.getPoolCandidates(ctx, nic, defaultIPV4IPPool, defaultIPV6IPPool, cleanGateway, pod, podController)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IPPool candidates: %s", preliminary)

	if err := i.config.checkIPVersionEnable(ctx, preliminary); err != nil {
		return nil, err
	}

	if err := i.filterPoolCandidates(ctx, preliminary, pod); err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Filtered IPPool candidates: %s", preliminary)

	if err := i.verifyPoolCandidates(ctx, preliminary); err != nil {
		return nil, err
	}
	logger.Info("All IPPool candidates valid")

	return preliminary, nil
}

func (i *ipam) allocateForAllNICs(ctx context.Context, tt []*ToBeAllocated, containerID string, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	customRoutes, err := getCustomRoutes(ctx, pod)
	if err != nil {
		return nil, err
	}

	var allResults []*AllocationResult
	for _, t := range tt {
		oneResults, err := i.allocateForOneNIC(ctx, t, containerID, &customRoutes, endpoint, pod)
		if len(oneResults) != 0 {
			allResults = append(allResults, oneResults...)
		}
		if err != nil {
			return allResults, err
		}
	}
	if len(customRoutes) != 0 {
		logger.Sugar().Warnf("Invalid custom routes: %v", customRoutes)
	}

	ips, _ := convertResultsToIPConfigsAndAllRoutes(allResults)
	anno, err := genIPAssignmentAnnotation(ips)
	if err != nil {
		return allResults, fmt.Errorf("failed to generate IP assignment annotation: %v", err)
	}

	if err := i.podManager.MergeAnnotations(ctx, pod.Namespace, pod.Name, anno); err != nil {
		return allResults, fmt.Errorf("failed to merge IP assignment annotation: %w", err)
	}

	return allResults, nil
}

func (i *ipam) allocateForOneNIC(ctx context.Context, t *ToBeAllocated, containerID string, customRoutes *[]*models.Route, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) ([]*AllocationResult, error) {
	var results []*AllocationResult
	for _, c := range t.PoolCandidates {
		result, err := i.allocateIPFromPoolCandidates(ctx, c, t.NIC, containerID, t.CleanGateway, pod)
		if err != nil {
			return results, err
		}
		results = append(results, result)

		routes, err := groupCustomRoutesByGW(ctx, customRoutes, result.IP)
		if err != nil {
			return results, fmt.Errorf("failed to group custom routes by gateway: %v", err)
		}
		result.Routes = append(result.Routes, routes...)

		patch := convertResultsToIPDetails([]*AllocationResult{result})
		if err = i.weManager.PatchIPAllocation(ctx, &spiderpoolv1.PodIPAllocation{
			ContainerID: containerID,
			IPs:         patch,
		}, endpoint); err != nil {
			return results, fmt.Errorf("failed to patch IP allocation detail %+v to Endpoint %s/%s: %v", patch, endpoint.Namespace, endpoint.Name, err)
		}
	}

	return results, nil
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
	var result *AllocationResult
	for _, pool := range c.Pools {
		ip, ipPool, err := i.ipPoolManager.AllocateIP(ctx, pool, containerID, nic, pod)
		if err != nil {
			errs = append(errs, err)
			logger.Sugar().Warnf("Failed to allocate IPv%d IP to %s from IPPool %s: %v", c.IPVersion, nic, pool, err)
			continue
		}

		result = &AllocationResult{
			IP:           ip,
			CleanGateway: cleanGateway,
			Routes:       convertSpecRoutesToOAIRoutes(nic, ipPool.Spec.Routes),
		}
		logger.Sugar().Infof("Allocate IPv%d IP %s to %s from IPPool %s", c.IPVersion, *result.IP.Address, nic, pool)
		break
	}

	if len(errs) == len(c.Pools) {
		return nil, fmt.Errorf("failed to allocate any IPv%d IP address to %s from IPPools %v: %w", c.IPVersion, nic, c.Pools, utilerrors.NewAggregate(errs))
	}

	return result, nil
}

func (i *ipam) getPoolCandidates(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, cleanGateway bool, pod *corev1.Pod, podController types.PodTopController) ([]*ToBeAllocated, error) {
	// Select candidate IPPools through the Pod annotations "ipam.spidernet.io/subnets" or "ipam.spidernet.io/subnet"
	// if we enable to use SpiderSubnet feature
	if i.config.EnableSpiderSubnet {
		fromSubnet, err := i.getPoolFromSubnetAnno(ctx, pod, nic, cleanGateway, podController)
		if nil != err {
			return nil, fmt.Errorf("failed to get IPPool from SpiderSubnet, error: %v", err)
		}
		if fromSubnet != nil {
			return []*ToBeAllocated{fromSubnet}, nil
		}
	}

	// Select candidate IPPools through the Pod annotation "ipam.spidernet.io/ippools".
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		return getPoolFromPodAnnoPools(ctx, anno, nic)
	}

	// Select candidate IPPools through the Pod annotation "ipam.spidernet.io/ippool".
	if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		t, err := getPoolFromPodAnnoPool(ctx, anno, nic, cleanGateway)
		if err != nil {
			return nil, err
		}
		return []*ToBeAllocated{t}, nil
	}

	// Select candidate IPPools through Cluster Default Subnet with Configmap Spiderpool-conf
	// if we enable to use SpiderSubnet feature
	if i.config.EnableSpiderSubnet {
		fromClusterDefaultSubnet, err := i.getPoolFromClusterDefaultSubnet(ctx, pod, nic, cleanGateway, podController)
		if nil != err {
			return nil, err
		}
		if fromClusterDefaultSubnet != nil {
			return []*ToBeAllocated{fromClusterDefaultSubnet}, nil
		}
	}

	// Select candidate IPPools through the Namespace annotations
	// "ipam.spidernet.io/defaultv4ippool" and "ipam.spidernet.io/defaultv6ippool".
	t, err := i.getPoolFromNS(ctx, pod.Namespace, nic, cleanGateway)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return []*ToBeAllocated{t}, nil
	}

	// Select candidate IPPools through CNI network configuration.
	if t := getPoolFromNetConf(ctx, nic, netConfV4Pool, netConfV6Pool, cleanGateway); t != nil {
		return []*ToBeAllocated{t}, nil
	}

	// Select candidate IPPools through Configmap spiderpool-conf.
	t, err = i.config.getClusterDefaultPool(ctx, nic, cleanGateway)
	if err != nil {
		return nil, err
	}

	return []*ToBeAllocated{t}, nil
}

func (i *ipam) getPoolFromSubnetAnno(ctx context.Context, pod *corev1.Pod, nic string, cleanGateway bool, podController types.PodTopController) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	// get SpiderSubnet configuration from pod annotation
	subnetAnnoConfig, err := subnetmanagercontrollers.GetSubnetAnnoConfig(pod.Annotations, logger)
	if nil != err {
		return nil, err
	}

	// default IPPool mode
	if subnetmanagercontrollers.IsDefaultIPPoolMode(subnetAnnoConfig) {
		return nil, nil
	}

	var subnetItem types.AnnoSubnetItem
	if len(subnetAnnoConfig.MultipleSubnets) != 0 {
		for index := range subnetAnnoConfig.MultipleSubnets {
			if subnetAnnoConfig.MultipleSubnets[index].Interface == nic {
				subnetItem = subnetAnnoConfig.MultipleSubnets[index]
				break
			}
		}
	} else if subnetAnnoConfig.SingleSubnet != nil {
		subnetItem = *subnetAnnoConfig.SingleSubnet
	} else {
		return nil, fmt.Errorf("there are no subnet specified: %v", subnetAnnoConfig)
	}

	if i.config.EnableIPv4 && len(subnetItem.IPv4) == 0 {
		return nil, fmt.Errorf("the pod subnetAnnotation doesn't specify IPv4 SpiderSubnet")
	}
	if i.config.EnableIPv6 && len(subnetItem.IPv6) == 0 {
		return nil, fmt.Errorf("the pod subnetAnnotation doesn't specify IPv6 SpiderSubnet")
	}

	result := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}

	// This only serves for orphan pod or third party controller application, because we'll create or scale the auto-created IPPool here.
	// For those kubernetes applications(such as deployment and replicaset), the spiderpool-controller will create or scale the auto-created IPPool asynchronously.
	poolIPNum, podSelector, err := getAutoPoolIPNumberAndSelector(pod, podController)
	if nil != err {
		return nil, err
	}

	// This function will find the IPPool with the given match labels.
	// The first return parameter represents the IPPool name, and the second parameter represents whether you need to create IPPool for orphan pod.
	// If the application is an orphan pod and do not find any IPPool, it will return immediately to inform you to create IPPool.
	findSubnetIPPool := func(matchLabels client.MatchingLabels) (string, bool, error) {
		var poolName string
		subnetName := matchLabels[constant.LabelIPPoolOwnerSpiderSubnet]
		for j := 0; j <= i.config.OperationRetries; j++ {
			poolList, err := i.ipPoolManager.ListIPPools(ctx, matchLabels)
			if nil != err {
				return "", false, fmt.Errorf("failed to get IPPoolList with labels '%v', error: %v", matchLabels, err)
			}

			// validation
			if poolList == nil || len(poolList.Items) == 0 {
				// the orphan pod should create its auto IPPool immediately if no IPPool found
				if podController.Kind == constant.KindPod || podController.Kind == constant.KindUnknown {
					return "", true, nil
				}

				logger.Sugar().Errorf("no '%s' IPPool retrieved from SpiderSubnet '%s', wait for a second and get a retry",
					matchLabels[constant.LabelIPPoolVersion], subnetName)
				time.Sleep(i.config.OperationGapDuration)
				continue
			} else if len(poolList.Items) == 1 {
				// check whether the auto IPPool need to scale it desiredIPNumber or not for orphan pod and third party controller application
				if podController.Kind == constant.KindPod || podController.Kind == constant.KindUnknown {
					pool := poolList.Items[0].DeepCopy()
					logger.Sugar().Debugf("found SpiderSubnet '%s' IPPool '%s' and check it whether need to be scaled", subnetName, pool.Name)
					err := i.subnetManager.CheckScaleIPPool(ctx, pool, subnetName, poolIPNum)
					if nil != err {
						return "", false, fmt.Errorf("failed to check IPPool %s whether need to be scaled: %v", pool.Name, err)
					}
				}
			} else {
				return "", false, fmt.Errorf("it's invalid for '%s/%s/%s' corresponding SpiderSubnet '%s' owns multiple IPPools '%v' for one specify application",
					podController.Kind, podController.Namespace, podController.Name, subnetName, poolList.Items)
			}

			poolName = poolList.Items[0].Name
			break
		}
		if len(poolName) == 0 {
			return "", false, fmt.Errorf("no matching IPPool candidate with labels '%v'", matchLabels)
		}
		return poolName, false, nil
	}

	var v4PoolCandidate, v6PoolCandidate string
	// we create auto-created IPPool for orphan pod or third party controller application here
	var shouldCreateV4Pool, shouldCreateV6Pool bool
	var errV4, errV6 error
	var wg sync.WaitGroup

	// get pod annotation "ipam.spidernet.io/reclaim-ippool"
	reclaimIPPool, err := subnetmanagercontrollers.ShouldReclaimIPPool(pod.Annotations)
	if nil != err {
		return nil, err
	}
	// we don't support reclaim IPPool for third party controller application
	if podController.Kind == constant.KindUnknown {
		reclaimIPPool = false
	}

	// if enableIPv4 is off and get the specified SpiderSubnet IPv4 name, just filter it out
	if i.config.EnableIPv4 && len(subnetItem.IPv4) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			v4PoolCandidate, shouldCreateV4Pool, errV4 = findSubnetIPPool(client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(podController.Uid),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV4,
				constant.LabelIPPoolOwnerSpiderSubnet:   subnetItem.IPv4[0],
				constant.LabelIPPoolOwnerApplication:    subnetmanagercontrollers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
				constant.LabelIPPoolInterface:           subnetItem.Interface,
			})
			if nil != errV4 {
				return
			}

			if shouldCreateV4Pool {
				v4Pool, err := i.subnetManager.AllocateEmptyIPPool(ctx, subnetItem.IPv4[0], podController, podSelector, poolIPNum, constant.IPv4, reclaimIPPool, nic)
				if nil != err {
					errV4 = err
					return
				}
				v4PoolCandidate = v4Pool.Name
				// wait for a second to make the spiderpool-controller allocates IP for the IPPool from SpiderSubnet
				time.Sleep(i.config.OperationGapDuration)
			}
		}()
	}

	// if enableIPv6 is off and get the specified SpiderSubnet IPv6 name, just filter it out
	if i.config.EnableIPv6 && len(subnetItem.IPv6) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			v6PoolCandidate, shouldCreateV6Pool, errV6 = findSubnetIPPool(client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(podController.Uid),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV6,
				constant.LabelIPPoolOwnerSpiderSubnet:   subnetItem.IPv6[0],
				constant.LabelIPPoolOwnerApplication:    subnetmanagercontrollers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
				constant.LabelIPPoolInterface:           subnetItem.Interface,
			})
			if nil != errV6 {
				return
			}

			if shouldCreateV6Pool {
				v6Pool, err := i.subnetManager.AllocateEmptyIPPool(ctx, subnetItem.IPv6[0], podController, podSelector, poolIPNum, constant.IPv6, reclaimIPPool, nic)
				if nil != err {
					errV6 = err
					return
				}
				v6PoolCandidate = v6Pool.Name
				// wait for a second to make the spiderpool-controller allocates IP for the IPPool from SpiderSubnet
				time.Sleep(i.config.OperationGapDuration)
			}
		}()
	}

	wg.Wait()

	if errV4 != nil || errV6 != nil {
		return nil, multierr.Append(errV4, errV6)
	}

	if v4PoolCandidate != "" {
		logger.Sugar().Debugf("add IPv4 subnet IPPool '%s' to PoolCandidates", v4PoolCandidate)
		result.PoolCandidates = append(result.PoolCandidates, &PoolCandidate{IPVersion: constant.IPv4, Pools: []string{v4PoolCandidate}})
	}
	if v6PoolCandidate != "" {
		logger.Sugar().Debugf("add IPv6 subnet IPPool '%s' to PoolCandidates", v6PoolCandidate)
		result.PoolCandidates = append(result.PoolCandidates, &PoolCandidate{IPVersion: constant.IPv6, Pools: []string{v6PoolCandidate}})
	}

	return result, nil
}

func (i *ipam) getPoolFromClusterDefaultSubnet(ctx context.Context, pod *corev1.Pod, nic string, cleanGateway bool, podController types.PodTopController) (*ToBeAllocated, error) {
	log := logutils.FromContext(ctx)

	poolIPNum, podSelector, err := getAutoPoolIPNumberAndSelector(pod, podController)
	if nil != err {
		return nil, err
	}

	// get pod annotation "ipam.spidernet.io/reclaim-ippool"
	reclaimIPPool, err := subnetmanagercontrollers.ShouldReclaimIPPool(pod.Annotations)
	if nil != err {
		return nil, err
	}

	var v4PoolName, v6PoolName string
	for j := 0; j <= i.config.OperationRetries; j++ {
		v4PoolName, v6PoolName, err = i.findOrApplyClusterSubnetDefaultIPPool(ctx, podController, podSelector, nic, poolIPNum, reclaimIPPool)
		if nil != err {
			if j == i.config.OperationRetries {
				return nil, fmt.Errorf("exhaust all retries to find or apply auto-created IPPool: %v", err)
			}
			log.Sugar().Errorf("failed to find or apply auto-created IPPool with %d times: %v", j, err)
			time.Sleep(i.config.OperationGapDuration)
			continue
		}
		break
	}

	// no cluster default subnets
	if v4PoolName == "" && v6PoolName == "" {
		return nil, nil
	}

	result := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}

	if v4PoolName != "" {
		result.PoolCandidates = append(result.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     []string{v4PoolName},
		})
	}

	if v6PoolName != "" {
		result.PoolCandidates = append(result.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     []string{v6PoolName},
		})
	}

	return result, nil
}

// findOrApplyClusterSubnetDefaultIPPool serves for cluster default subnet usage.
// This will create auto-created IPPool or update auto-created IPPool desired IP number
func (i *ipam) findOrApplyClusterSubnetDefaultIPPool(ctx context.Context, podController types.PodTopController, podSelector *metav1.LabelSelector, ifName string, poolIPNum int, reclaimIPPool bool) (v4PoolName, v6PoolName string, err error) {
	log := logutils.FromContext(ctx)

	var clusterDefaultV4Subnet, clusterDefaultV6Subnet string

	if len(singletons.ClusterDefaultPool.ClusterDefaultIPv4Subnet) != 0 {
		clusterDefaultV4Subnet = singletons.ClusterDefaultPool.ClusterDefaultIPv4Subnet[0]
	}
	if len(singletons.ClusterDefaultPool.ClusterDefaultIPv6Subnet) != 0 {
		clusterDefaultV6Subnet = singletons.ClusterDefaultPool.ClusterDefaultIPv6Subnet[0]
	}
	// no cluster default subnet specified
	if (i.config.EnableIPv4 && clusterDefaultV4Subnet == "") || (i.config.EnableIPv6 && clusterDefaultV6Subnet == "") {
		return "", "", nil
	}

	fn := func(poolList *spiderpoolv1.SpiderIPPoolList, subnetName string, ipVersion types.IPVersion) (string, error) {
		if poolList == nil || len(poolList.Items) == 0 {
			log.Sugar().Debugf("there's no 'IPv%d' IPPoolList retrieved from cluster default SpiderSubent '%s'", ipVersion, subnetName)
			pool, err := i.subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, poolIPNum, ipVersion, reclaimIPPool, ifName)
			if nil != err {
				return "", err
			}
			return pool.Name, nil
		} else if len(poolList.Items) == 1 {
			pool := poolList.Items[0].DeepCopy()
			log.Sugar().Debugf("found cluster default SpiderSubnet '%s' IPPool '%s' and check it whether need to be scaled", subnetName, pool.Name)
			err := i.subnetManager.CheckScaleIPPool(ctx, pool, subnetName, poolIPNum)
			if nil != err {
				return "", err
			}
			return pool.Name, nil
		} else {
			return "", fmt.Errorf("%w: it's invalid that SpiderSubnet '%s' owns multiple IPPools '%v' for one specify application", constant.ErrWrongInput, subnetName, poolList.Items)
		}
	}

	var errV4, errV6 error
	var wg sync.WaitGroup

	if i.config.EnableIPv4 && clusterDefaultV4Subnet != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			matchLabels := client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(podController.Uid),
				constant.LabelIPPoolOwnerSpiderSubnet:   clusterDefaultV4Subnet,
				constant.LabelIPPoolOwnerApplication:    subnetmanagercontrollers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV4,
				constant.LabelIPPoolInterface:           ifName,
			}
			v4PoolList, err := i.ipPoolManager.ListIPPools(ctx, matchLabels)
			if nil != err {
				errV4 = fmt.Errorf("failed to get IPv4 IPPoolList with labels '%v', error: %v", matchLabels, err)
				return
			}

			v4PoolName, errV4 = fn(v4PoolList.DeepCopy(), clusterDefaultV4Subnet, constant.IPv4)
		}()
	}

	if i.config.EnableIPv6 && clusterDefaultV6Subnet != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			matchLabels := client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(podController.Uid),
				constant.LabelIPPoolOwnerSpiderSubnet:   clusterDefaultV6Subnet,
				constant.LabelIPPoolOwnerApplication:    subnetmanagercontrollers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV6,
				constant.LabelIPPoolInterface:           ifName,
			}
			v6PoolList, err := i.ipPoolManager.ListIPPools(ctx, matchLabels)
			if nil != err {
				errV6 = fmt.Errorf("failed to get IPv6 IPPoolList with labels '%v', error: %v", matchLabels, err)
				return
			}

			v6PoolName, errV6 = fn(v6PoolList.DeepCopy(), clusterDefaultV6Subnet, constant.IPv6)
		}()
	}

	wg.Wait()

	if errV4 != nil || errV6 != nil {
		return "", "", multierr.Append(errV4, errV6)
	}

	return
}

func (i *ipam) getPoolFromNS(ctx context.Context, namespace, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	ns, err := i.nsManager.GetNamespaceByName(ctx, namespace)
	if err != nil {
		return nil, err
	}
	nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(ns)
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

func (i *ipam) filterPoolCandidates(ctx context.Context, tt []*ToBeAllocated, pod *corev1.Pod) error {
	logger := logutils.FromContext(ctx)

	for _, t := range tt {
		for _, c := range t.PoolCandidates {
			var errs []error
			var selectedPools []string
			for _, pool := range c.Pools {
				if err := i.selectByPod(ctx, c.IPVersion, pool, pod); err != nil {
					errs = append(errs, err)
					logger.Sugar().Warnf("IPPool %s is filtered by Pod %s/%s: %v", pool, pod.Namespace, pod.Name, err)
					continue
				}
				selectedPools = append(selectedPools, pool)
			}

			if len(errs) == len(c.Pools) {
				return fmt.Errorf("%w, all IPv%d IPPools %v of %s filtered out: %v", constant.ErrNoAvailablePool, c.IPVersion, c.Pools, t.NIC, utilerrors.NewAggregate(errs))
			}
			c.Pools = selectedPools
		}
	}

	return nil
}

func (i *ipam) selectByPod(ctx context.Context, version types.IPVersion, poolName string, pod *corev1.Pod) error {
	ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, poolName)
	if err != nil {
		return fmt.Errorf("failed to get IPPool %s: %v", poolName, err)
	}

	if ipPool.DeletionTimestamp != nil {
		return fmt.Errorf("terminating IPPool %s", poolName)
	}

	if *ipPool.Spec.Disable {
		return fmt.Errorf("disabled IPPool %s", poolName)
	}

	if *ipPool.Spec.IPVersion != version {
		return fmt.Errorf("expect an IPv%d IPPool, but the version of the IPPool %s is IPv%d", version, poolName, *ipPool.Spec.IPVersion)
	}

	if ipPool.Status.TotalIPCount != nil && ipPool.Status.AllocatedIPCount != nil {
		if *ipPool.Status.TotalIPCount-*ipPool.Status.AllocatedIPCount == 0 {
			return constant.ErrIPUsedOut
		}
	}

	if ipPool.Spec.NodeAffinity != nil {
		nodeMatched, err := i.nodeManager.MatchLabelSelector(ctx, pod.Spec.NodeName, ipPool.Spec.NodeAffinity)
		if err != nil {
			return fmt.Errorf("failed to check the Node affinity of IPPool %s: %v", poolName, err)
		}
		if !nodeMatched {
			return fmt.Errorf("unmatched Node affinity of IPPool %s", poolName)
		}
	}

	if ipPool.Spec.NamespaceAffinity != nil {
		nsMatched, err := i.nsManager.MatchLabelSelector(ctx, pod.Namespace, ipPool.Spec.NamespaceAffinity)
		if err != nil {
			return fmt.Errorf("failed to check the Namespace affinity of IPPool %s: %v", poolName, err)
		}
		if !nsMatched {
			return fmt.Errorf("unmatched Namespace affinity of IPPool %s", poolName)
		}
	}

	if ipPool.Spec.PodAffinity != nil {
		podMatched, err := i.podManager.MatchLabelSelector(ctx, pod.Namespace, pod.Name, ipPool.Spec.PodAffinity)
		if err != nil {
			return fmt.Errorf("failed to check the Pod affinity of IPPool %s: %v", poolName, err)
		}
		if !podMatched {
			return fmt.Errorf("unmatched Pod affinity of IPPool %s", poolName)
		}
	}

	return nil
}

// TODO(iiiceoo): Refactor
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
	logger.Info("Start to release")

	endpoint, err := i.weManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get Endpoint %s/%s: %v", *delArgs.PodNamespace, *delArgs.PodName, err)
	}
	allocation, currently := workloadendpointmanager.RetrieveIPAllocation(*delArgs.ContainerID, *delArgs.IfName, true, endpoint)
	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
	} else {
		if !currently {
			logger.Warn("Request to release non current IP allocation, concurrency may exist between the same Pod")
		}

		// check whether the StatefulSet pod need to release
		// ref: https://github.com/spidernet-io/spiderpool/issues/1045
		if i.config.EnableStatefulSet && endpoint.Status.OwnerControllerType == constant.KindStatefulSet {
			shouldCleanSts, err := i.shouldReleaseStatefulSet(ctx, endpoint.Namespace, endpoint.Name, allocation, currently)
			if nil != err {
				return err
			}
			if !shouldCleanSts {
				logger.Info("No need to release the IP allocation of an StatefulSet whose scale is not reduced")
				return nil
			}
		}

		if err = i.release(ctx, allocation.ContainerID, allocation.IPs); err != nil {
			return err
		}
		if err := i.weManager.ClearCurrentIPAllocation(ctx, *delArgs.ContainerID, endpoint); err != nil {
			return fmt.Errorf("failed to clear current IP allocation: %v", err)
		}
		logger.Sugar().Infof("Succeed to release: %+v", allocation.IPs)
	}

	// this serves for orphan pod if SpiderSubnet feature is enabled
	if i.config.EnableSpiderSubnet {
		logger.Info("try to check whether need to delete dead orphan pod's auto-created IPPool")
		err = i.deleteDeadOrphanPodAutoIPPool(ctx, *delArgs.PodNamespace, *delArgs.PodName, *delArgs.IfName)
		if nil != err {
			return fmt.Errorf("failed to delete dead orphan pod auto-created IPPool: %v", err)
		}
	}

	return nil
}

// shouldReleaseStatefulSet checks whether the StatefulSet pod need to be released, if the StatefulSet object was deleted or decreased its replicas.
// And we'll also check whether the StatefulSet pod's last ipam allocation is invalid or not,
// if we set dual stack but only get one IP allocation, we should clean up it.
func (i *ipam) shouldReleaseStatefulSet(ctx context.Context, podNamespace, podName string, allocation *spiderpoolv1.PodIPAllocation, currently bool) (bool, error) {
	log := logutils.FromContext(ctx)

	isValidStsPod, err := i.stsManager.IsValidStatefulSetPod(ctx, podNamespace, podName, constant.KindStatefulSet)
	if err != nil {
		return false, err
	}

	// StatefulSet deleted or replicas decreased
	if !isValidStsPod {
		return true, nil
	}

	// the last allocation is failed, try to clean up all allocation and re-allocate in the next time
	if currently {
		for index := range allocation.IPs {
			if i.config.EnableIPv4 && allocation.IPs[index].IPv4 == nil ||
				i.config.EnableIPv6 && allocation.IPs[index].IPv6 == nil {
				log.Sugar().Warnf("StatefulSet pod has legacy failure allocation %v", allocation.IPs[index])
				return true, nil
			}
		}
	}

	return false, nil
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
			logger.Sugar().Infof("Succeed to release IP address %+v from IPPool %s", ipAndCIDs, pool)
		}(pool, ipAndCIDs)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to release all allocated IP addresses %+v: %w", poolToIPAndCIDs, utilerrors.NewAggregate(errs))
	}

	return nil
}

func (i *ipam) Start(ctx context.Context) error {
	return i.ipamLimiter.Start(ctx)
}

// deleteDeadOrphanPodAutoIPPool will delete orphan pod corresponding IPPools
func (i *ipam) deleteDeadOrphanPodAutoIPPool(ctx context.Context, podNS, podName, ifName string) error {
	log := logutils.FromContext(ctx)

	isAlive := true
	pod, err := i.podManager.GetPodByName(ctx, podNS, podName)
	if nil != err {
		if apierrors.IsNotFound(err) {
			isAlive = false
			log.Debug("pod is already deleted, try to delete its auto-created IPPool")
		} else {
			return fmt.Errorf("failed to get pod: %v", err)
		}
	} else {
		if pod.DeletionTimestamp != nil {
			isAlive = false
			log.Debug("pod is terminating, try to delete its auto-created IPPool")
		}
	}

	if !isAlive {
		matchLabels := client.MatchingLabels{
			// this label make it sure to find orphan pod corresponding IPPool
			constant.LabelIPPoolOwnerApplication: subnetmanagercontrollers.AppLabelValue(constant.KindPod, podNS, podName),
			// TODO(Icarus9913): should we delete all interfaces auto-created IPPool in the first cmdDel?
			constant.LabelIPPoolInterface:     ifName,
			constant.LabelIPPoolReclaimIPPool: constant.True,
		}
		poolList, err := i.ipPoolManager.ListIPPools(ctx, matchLabels)
		if nil != err {
			return fmt.Errorf("failed to get IPPoolList with the given label '%v', error: %v", matchLabels, err)
		}

		log.Sugar().Infof("found orphan pod corresponding IPPool list: %v, try to delete them", poolList.Items)
		for index := range poolList.Items {
			err = i.ipPoolManager.DeleteIPPool(ctx, &poolList.Items[index])
			if nil != err {
				return err
			}
		}
	}

	return nil
}
