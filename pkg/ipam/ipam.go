// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/singletons"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	subnetmanagercontrollers "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

type IPAM interface {
	Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error)
	Release(ctx context.Context, delArgs *models.IpamDelArgs) error
	Start(ctx context.Context) error
}

type ipam struct {
	config      IPAMConfig
	ipamLimiter limiter.Limiter

	ipPoolManager   ippoolmanager.IPPoolManager
	endpointManager workloadendpointmanager.WorkloadEndpointManager
	nodeManager     nodemanager.NodeManager
	nsManager       namespacemanager.NamespaceManager
	podManager      podmanager.PodManager
	stsManager      statefulsetmanager.StatefulSetManager
	subnetManager   subnetmanager.SubnetManager

	rollbacks sync.Map
}

func NewIPAM(
	config IPAMConfig,
	ipPoolManager ippoolmanager.IPPoolManager,
	endpointManager workloadendpointmanager.WorkloadEndpointManager,
	nodeManager nodemanager.NodeManager,
	nsManager namespacemanager.NamespaceManager,
	podManager podmanager.PodManager,
	stsManager statefulsetmanager.StatefulSetManager,
	subnetManager subnetmanager.SubnetManager,
) (IPAM, error) {
	if ipPoolManager == nil {
		return nil, fmt.Errorf("ippool manager %w", constant.ErrMissingRequiredParam)
	}
	if endpointManager == nil {
		return nil, fmt.Errorf("endpoint manager %w", constant.ErrMissingRequiredParam)
	}
	if nodeManager == nil {
		return nil, fmt.Errorf("node manager %w", constant.ErrMissingRequiredParam)
	}
	if nsManager == nil {
		return nil, fmt.Errorf("namespace manager %w", constant.ErrMissingRequiredParam)
	}
	if podManager == nil {
		return nil, fmt.Errorf("pod manager %w", constant.ErrMissingRequiredParam)
	}
	if stsManager == nil {
		return nil, fmt.Errorf("statefulset manager %w", constant.ErrMissingRequiredParam)
	}
	if config.EnableSpiderSubnet && subnetManager == nil {
		return nil, fmt.Errorf("subnet manager %w", constant.ErrMissingRequiredParam)
	}

	return &ipam{
		config:          setDefaultsForIPAMConfig(config),
		ipamLimiter:     limiter.NewLimiter(config.LimiterConfig),
		ipPoolManager:   ipPoolManager,
		endpointManager: endpointManager,
		nodeManager:     nodeManager,
		nsManager:       nsManager,
		podManager:      podManager,
		stsManager:      stsManager,
		subnetManager:   subnetManager,
		rollbacks:       sync.Map{},
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
		return nil, fmt.Errorf("%s Pod %s/%s cannot allocate IP addresees", strings.ToLower(string(podStatus)), pod.Namespace, pod.Name)
	}
	logger.Sugar().Debugf("Get Pod with status %s", podStatus)

	podTopController, err := i.podManager.GetPodTopController(ctx, pod)
	if nil != err {
		return nil, fmt.Errorf("failed to get the top controller of the Pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
	logger.Sugar().Debugf("%s %s/%s is the top controller of the Pod", podTopController.Kind, podTopController.Namespace, podTopController.Name)

	endpoint, err := i.endpointManager.GetEndpointByName(ctx, pod.Namespace, pod.Name)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get Endpoint %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	if endpoint == nil {
		logger.Sugar().Debugf("Get Endpoint %s/%s", pod.Namespace, pod.Name)
	} else {
		logger.Debug("No Endpoint")
	}

	if i.config.EnableStatefulSet && podTopController.Kind == constant.KindStatefulSet {
		logger.Info("Retrieve the IP allocation of StatefulSet")
		addResp, err := i.retrieveStsIPAllocation(ctx, *addArgs.ContainerID, pod, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation of StatefulSet %s/%s: %w", podTopController.Namespace, podTopController.Name, err)
		}
		if addResp != nil {
			return addResp, nil
		}
	} else {
		logger.Debug("Try to retrieve the existing IP allocation in multi-NIC mode")
		addResp, err := i.retrieveMultiNICIPAllocation(ctx, *addArgs.ContainerID, *addArgs.IfName, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation in multi-NIC mode: %w", err)
		}
		if addResp != nil {
			return addResp, nil
		}
	}

	logger.Info("Allocate IP addresses in standard mode")
	addResp, err := i.allocateInStandardMode(ctx, addArgs, pod, endpoint, podTopController)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP addresses in standard mode: %w", err)
	}

	return addResp, nil
}

func (i *ipam) retrieveStsIPAllocation(ctx context.Context, containerID string, pod *corev1.Pod, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	// The first allocation.
	if endpoint == nil {
		logger.Debug("Endpoint not found, try to allocate IP in standard mode instead of retrieving")
		return nil, nil
	}

	// The rollback records is missing and the last allocation failed.
	if endpoint.Status.Current == nil {
		logger.Warn("Endpoint doesn't have current IP allocation, try to re-allocate")
		return nil, nil
	}

	// Failed to allocate any IP addresses in the last allocation.
	if len(endpoint.Status.Current.IPs) == 0 {
		return nil, nil
	}

	// Check again in case cmdDel() cannot be complete under some special
	// circumstances. Or the IP version config of spiderpool is modified.
	for _, d := range endpoint.Status.Current.IPs {
		if i.config.EnableIPv4 && d.IPv4 == nil ||
			i.config.EnableIPv6 && d.IPv6 == nil {
			return nil, fmt.Errorf("the Pod of StatefulSet has legacy failure allocation %+v", d)
		}
	}

	// Concurrently refresh the IP records of the IPPools.
	if err := i.reallocateIPPoolIPRecords(ctx, containerID, pod.Spec.NodeName, endpoint); err != nil {
		return nil, err
	}

	// Refresh the current IP allocation of the Endpoint.
	if err := i.endpointManager.ReallocateCurrentIPAllocation(ctx, containerID, pod.Spec.NodeName, endpoint); err != nil {
		return nil, fmt.Errorf("failed to update the current IP allocation of StatefulSet: %w", err)
	}

	ips, routes := convertIPDetailsToIPConfigsAndAllRoutes(endpoint.Status.Current.IPs)
	addResp := &models.IpamAddResponse{
		Ips:    ips,
		Routes: routes,
	}
	logger.Sugar().Infof("Succeed to retrieve the IP allocation of StatefulSet: %+v", *addResp)

	return addResp, nil
}

func (i *ipam) reallocateIPPoolIPRecords(ctx context.Context, containerID, nodeName string, endpoint *spiderpoolv1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)

	pics := GroupIPDetails(containerID, nodeName, endpoint.Status.Current.IPs)
	tickets := pics.Pools()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return fmt.Errorf("failed to queue correctly: %v", err)
	}
	defer i.ipamLimiter.ReleaseTicket(ctx, tickets...)

	errCh := make(chan error, len(pics))
	wg := sync.WaitGroup{}
	wg.Add(len(pics))

	for p, ics := range pics {
		go func(poolName string, ipAndCIDs []types.IPAndCID) {
			defer wg.Done()

			if err := i.ipPoolManager.UpdateAllocatedIPs(ctx, poolName, ipAndCIDs); err != nil {
				logger.Warn(err.Error())
				errCh <- err
				return
			}
			logger.Sugar().Infof("Succeed to re-allocate IP addresses %+v from IPPool %s", ipAndCIDs, poolName)
		}(p, ics)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to re-allocate all allocated IP addresses %+v: %w", pics, utilerrors.NewAggregate(errs))
	}

	return nil
}

func (i *ipam) retrieveMultiNICIPAllocation(ctx context.Context, containerID, nic string, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	allocation := workloadendpointmanager.RetrieveIPAllocation(containerID, nic, endpoint)
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

	logger.Debug("Parse custom routes")
	customRoutes, err := getCustomRoutes(pod)
	if err != nil {
		return nil, err
	}

	logger.Debug("Generate IPPool candidates")
	toBeAllocatedSet, err := i.genToBeAllocatedSet(ctx, addArgs, pod, podController)
	if err != nil {
		return nil, err
	}

	// TODO(iiiceoo): Comment why containerID should be written first.
	if endpoint == nil {
		logger.Sugar().Infof("First sandbox of Pod is being created, mark the IP allocation")
		endpoint, err = i.endpointManager.MarkIPAllocation(ctx, *addArgs.ContainerID, pod, podController)
		if err != nil {
			return nil, fmt.Errorf("failed to mark IP allocation: %v", err)
		}
	} else {
		logger.Sugar().Infof("Sandbox has changed, remarking the IP allocation with the new container ID")
		if err := i.endpointManager.ReMarkIPAllocation(ctx, *addArgs.ContainerID, endpoint, pod); err != nil {
			return nil, fmt.Errorf("failed to remark IP allocation: %v", err)
		}
	}

	results, err := i.allocateForAllNICs(ctx, toBeAllocatedSet, *addArgs.ContainerID, customRoutes, endpoint, pod, podController)
	if err != nil {
		if len(results) != 0 {
			logger.Sugar().Warnf("Failed to allocate IP addresses for all NICs, record incomplete IP allocation results for rollback: %+v", results)
			i.addRollback(*addArgs.ContainerID, results)
		}
		return nil, err
	}

	resIPs, resRoutes := convertResultsToIPConfigsAndAllRoutes(results)
	addResp := &models.IpamAddResponse{
		Ips:    resIPs,
		Routes: resRoutes,
	}
	logger.Sugar().Infof("Succeed to allocate: %+v", *addResp)

	return addResp, nil
}

func (i *ipam) genToBeAllocatedSet(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, podController types.PodTopController) (ToBeAllocateds, error) {
	logger := logutils.FromContext(ctx)

	logger.Debug("Select original IPPools through pool selection rules")
	preliminary, err := i.getPoolCandidates(ctx, addArgs, pod, podController)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IPPool candidates: %s", preliminary)

	logger.Debug("Precheck IPPool candidates")
	if err := i.precheckPoolCandidates(ctx, preliminary); err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Prechecked IPPool candidates: %s", preliminary)

	logger.Debug("Filter out IPPool candidates")
	if err := i.filterPoolCandidates(ctx, preliminary, pod); err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Filtered IPPool candidates: %s", preliminary)

	logger.Debug("Verify IPPool candidates")
	if err := i.verifyPoolCandidates(preliminary); err != nil {
		return nil, err
	}
	logger.Info("All IPPool candidates are valid")

	return preliminary, nil
}

func (i *ipam) allocateForAllNICs(ctx context.Context, tt ToBeAllocateds, containerID string, customRoutes []*models.Route, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController) ([]*AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	logger.Sugar().Debugf("Concurrently allocate IP addresses from all IPPool candidates")
	results, err := i.allocateIPsFromAllCandidates(ctx, tt, containerID, pod, podController)
	if err != nil {
		return results, err
	}

	logger.Sugar().Debugf("Group custom routes by IP allocation results")
	if err := groupCustomRoutes(ctx, customRoutes, results); err != nil {
		return results, fmt.Errorf("failed to group custom routes %+v: %v", customRoutes, err)
	}

	logger.Sugar().Debugf("Patch IP allocation detail to Endpoint %s/%s", endpoint.Namespace, endpoint.Name)
	if err = i.endpointManager.PatchIPAllocation(ctx, &spiderpoolv1.PodIPAllocation{
		ContainerID: containerID,
		IPs:         convertResultsToIPDetails(results),
	}, endpoint); err != nil {
		return results, fmt.Errorf("failed to patch IP allocation detail to Endpoint %s/%s: %v", endpoint.Namespace, endpoint.Name, err)
	}

	return results, nil
}

func (i *ipam) allocateIPsFromAllCandidates(ctx context.Context, tt ToBeAllocateds, containerID string, pod *corev1.Pod, podController types.PodTopController) ([]*AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	tickets := tt.Pools()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return nil, fmt.Errorf("failed to queue correctly: %v", err)
	}
	defer i.ipamLimiter.ReleaseTicket(ctx, tickets...)

	n := len(tt.Candidates())
	resultCh := make(chan *AllocationResult, n)
	errCh := make(chan error, n)
	wg := sync.WaitGroup{}
	wg.Add(n)

	for _, t := range tt {
		for _, c := range t.PoolCandidates {
			go func(candidate *PoolCandidate, nic string, cleanGateway bool) {
				defer wg.Done()

				clogger := logger.With(zap.String(
					"AllocateHash",
					fmt.Sprintf("%s-%d-%v", nic, candidate.IPVersion, candidate.Pools),
				))

				clogger.Sugar().Debugf("Try to allocate IPv%d IP address to NIC %s from IPPools %v", candidate.IPVersion, nic, candidate.Pools)
				result, err := i.allocateIPFromCandidate(logutils.IntoContext(ctx, clogger), candidate, nic, containerID, cleanGateway, pod, podController)
				if err != nil {
					clogger.Warn(err.Error())
					errCh <- err
					return
				}

				resultCh <- result
			}(c, t.NIC, t.CleanGateway)
		}
	}
	wg.Wait()
	close(resultCh)
	close(errCh)

	var results []*AllocationResult
	for res := range resultCh {
		results = append(results, res)
	}

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return results, utilerrors.NewAggregate(errs)
	}

	return results, nil
}

func (i *ipam) allocateIPFromCandidate(ctx context.Context, c *PoolCandidate, nic, containerID string, cleanGateway bool, pod *corev1.Pod, podController types.PodTopController) (*AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	var errs []error
	var result *AllocationResult
	for _, pool := range c.Pools {
		ip, err := i.ipPoolManager.AllocateIP(ctx, pool, containerID, nic, pod, podController)
		if err != nil {
			logger.Sugar().Warnf("Failed to allocate IPv%d IP address to NIC %s from IPPool %s: %v", c.IPVersion, nic, pool, err)
			errs = append(errs, err)
			continue
		}

		result = &AllocationResult{
			IP:           ip,
			CleanGateway: cleanGateway,
			Routes:       convertSpecRoutesToOAIRoutes(nic, c.PToIPPool[pool].Spec.Routes),
		}
		logger.Sugar().Infof("Allocate IPv%d IP %s to NIC %s from IPPool %s", c.IPVersion, *result.IP.Address, nic, pool)
		break
	}

	if len(errs) == len(c.Pools) {
		return nil, fmt.Errorf("failed to allocate any IPv%d IP address to NIC %s from IPPools %v: %w", c.IPVersion, nic, c.Pools, utilerrors.NewAggregate(errs))
	}

	return result, nil
}

func (i *ipam) getPoolCandidates(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, podController types.PodTopController) (ToBeAllocateds, error) {
	// If faature SpiderSubnet is enabled, select IPPool candidates through the
	// Pod annotations "ipam.spidernet.io/subnet" or "ipam.spidernet.io/subnets".
	if i.config.EnableSpiderSubnet {
		fromSubnet, err := i.getPoolFromSubnetAnno(ctx, pod, *addArgs.IfName, addArgs.CleanGateway, podController)
		if nil != err {
			return nil, fmt.Errorf("failed to get IPPool candidates from Subnet: %v", err)
		}
		if fromSubnet != nil {
			return ToBeAllocateds{fromSubnet}, nil
		}
	}

	// Select IPPool candidates through the Pod annotation "ipam.spidernet.io/ippools".
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		return getPoolFromPodAnnoPools(ctx, anno, *addArgs.IfName)
	}

	// Select IPPool candidates through the Pod annotation "ipam.spidernet.io/ippool".
	if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		t, err := getPoolFromPodAnnoPool(ctx, anno, *addArgs.IfName, addArgs.CleanGateway)
		if err != nil {
			return nil, err
		}
		return ToBeAllocateds{t}, nil
	}

	// If faature SpiderSubnet is enabled, select IPPool candidates through the cluster
	// default Subnet defined in Configmap spiderpool-conf.
	if i.config.EnableSpiderSubnet {
		fromClusterDefaultSubnet, err := i.getPoolFromClusterDefaultSubnet(ctx, pod, *addArgs.IfName, addArgs.CleanGateway, podController)
		if nil != err {
			return nil, err
		}
		if fromClusterDefaultSubnet != nil {
			return ToBeAllocateds{fromClusterDefaultSubnet}, nil
		}
	}

	// Select IPPool candidates through the Namespace annotations
	// "ipam.spidernet.io/defaultv4ippool" and "ipam.spidernet.io/defaultv6ippool".
	t, err := i.getPoolFromNS(ctx, pod.Namespace, *addArgs.IfName, addArgs.CleanGateway)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return ToBeAllocateds{t}, nil
	}

	// Select IPPool candidates through CNI network configuration.
	if t := getPoolFromNetConf(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, addArgs.CleanGateway); t != nil {
		return ToBeAllocateds{t}, nil
	}

	// Select IPPool candidates through Configmap spiderpool-conf.
	t, err = i.config.getClusterDefaultPool(ctx, *addArgs.IfName, addArgs.CleanGateway)
	if err != nil {
		return nil, err
	}

	return ToBeAllocateds{t}, nil
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
				constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
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
				constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
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
				constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
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
				constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
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

	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Namespace annotation '%s'", constant.AnnotationPre+"/default-ipv(4/6)-ippool")

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

func (i *ipam) precheckPoolCandidates(ctx context.Context, tt ToBeAllocateds) error {
	logger := logutils.FromContext(ctx)

	if err := i.config.checkIPVersionEnable(ctx, tt); err != nil {
		return err
	}

	// TODO(iiiceoo): Use gorouting and sync.Map.
	for _, t := range tt {
		for _, c := range t.PoolCandidates {
			ptp := PoolNameToIPPool{}
			for _, pool := range c.Pools {
				if _, ok := ptp[pool]; ok {
					return fmt.Errorf("%w, duplicate candidate IPPool %s", constant.ErrWrongInput, pool)
				}

				logger.Sugar().Debugf("Get original candidate IPPool %s", pool)
				ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, pool)
				if err != nil {
					return fmt.Errorf("failed to get original candidate IPPool %s: %v", pool, err)
				}
				ptp[pool] = ipPool
			}
			c.PToIPPool = ptp
		}
	}

	return nil
}

func (i *ipam) filterPoolCandidates(ctx context.Context, tt ToBeAllocateds, pod *corev1.Pod) error {
	logger := logutils.FromContext(ctx)

	for _, t := range tt {
		for _, c := range t.PoolCandidates {
			var errs []error

			for j := 0; j < len(c.Pools); j++ {
				pool := c.Pools[j]
				if err := i.selectByPod(ctx, c.IPVersion, c.PToIPPool[pool], pod); err != nil {
					logger.Sugar().Warnf("IPPool %s is filtered by Pod: %v", pool, err)
					errs = append(errs, err)

					delete(c.PToIPPool, pool)
					c.Pools = append((c.Pools)[:j], (c.Pools)[j+1:]...)
					j--
				}
			}

			if len(c.Pools) == 0 {
				return fmt.Errorf("%w, all IPv%d IPPools %v of %s filtered out: %v", constant.ErrNoAvailablePool, c.IPVersion, c.Pools, t.NIC, utilerrors.NewAggregate(errs))
			}
		}
	}

	return nil
}

func (i *ipam) selectByPod(ctx context.Context, version types.IPVersion, ipPool *spiderpoolv1.SpiderIPPool, pod *corev1.Pod) error {
	if ipPool.DeletionTimestamp != nil {
		return fmt.Errorf("terminating IPPool %s", ipPool.Name)
	}

	if *ipPool.Spec.Disable {
		return fmt.Errorf("disabled IPPool %s", ipPool.Name)
	}

	if *ipPool.Spec.IPVersion != version {
		return fmt.Errorf("expect an IPv%d IPPool, but the version of the IPPool %s is IPv%d", version, ipPool.Name, *ipPool.Spec.IPVersion)
	}

	if ipPool.Status.TotalIPCount != nil && ipPool.Status.AllocatedIPCount != nil {
		if *ipPool.Status.TotalIPCount-*ipPool.Status.AllocatedIPCount == 0 {
			return constant.ErrIPUsedOut
		}
	}

	if ipPool.Spec.NodeAffinity != nil {
		node, err := i.nodeManager.GetNodeByName(ctx, pod.Spec.NodeName)
		if err != nil {
			return err
		}
		selector, err := metav1.LabelSelectorAsSelector(ipPool.Spec.NodeAffinity)
		if err != nil {
			return err
		}
		if !selector.Matches(labels.Set(node.Labels)) {
			return fmt.Errorf("unmatched Node affinity of IPPool %s", ipPool.Name)
		}
	}

	if ipPool.Spec.NamespaceAffinity != nil {
		namespace, err := i.nsManager.GetNamespaceByName(ctx, pod.Namespace)
		if err != nil {
			return err
		}
		selector, err := metav1.LabelSelectorAsSelector(ipPool.Spec.NamespaceAffinity)
		if err != nil {
			return err
		}
		if !selector.Matches(labels.Set(namespace.Labels)) {
			return fmt.Errorf("unmatched Namespace affinity of IPPool %s", ipPool.Name)
		}
	}

	if ipPool.Spec.PodAffinity != nil {
		selector, err := metav1.LabelSelectorAsSelector(ipPool.Spec.PodAffinity)
		if err != nil {
			return err
		}
		if !selector.Matches(labels.Set(pod.Labels)) {
			return fmt.Errorf("unmatched Pod affinity of IPPool %s", ipPool.Name)
		}
	}

	return nil
}

func (i *ipam) verifyPoolCandidates(tt ToBeAllocateds) error {
	for _, t := range tt {
		var allIPPools []*spiderpoolv1.SpiderIPPool
		for _, c := range t.PoolCandidates {
			allIPPools = append(allIPPools, c.PToIPPool.IPPools()...)
		}

		vlanToPools := map[types.Vlan][]string{}
		for _, ipPool := range allIPPools {
			vlanToPools[*ipPool.Spec.Vlan] = append(vlanToPools[*ipPool.Spec.Vlan], ipPool.Name)
		}

		if len(vlanToPools) > 1 {
			return fmt.Errorf("%w, the VLANs of the IPPools corresponding to NIC %s are not all the same: %v", constant.ErrWrongInput, t.NIC, vlanToPools)
		}
	}

	// TODO(iiiceoo): Different NICs should not use IP address pertaining to
	// the same subnet.

	return nil
}

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to release")

	endpoint, err := i.endpointManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Endpoin does not exist, ignoring release")
			return nil
		}
		return fmt.Errorf("failed to get Endpoint %s/%s: %v", *delArgs.PodNamespace, *delArgs.PodName, err)
	}

	if err := i.releaseForAllNICs(ctx, *delArgs.ContainerID, *delArgs.IfName, endpoint); err != nil {
		return err
	}

	if i.config.EnableSpiderSubnet && endpoint.Status.OwnerControllerType == constant.KindPod {
		logger.Info("try to check whether need to delete dead orphan pod's auto-created IPPool")
		err := i.deleteDeadOrphanPodAutoIPPool(ctx, *delArgs.PodNamespace, *delArgs.PodName, *delArgs.IfName)
		if nil != err {
			logger.Sugar().Errorf("failed to delete dead orphan pod auto-created IPPool: %v", err)
		}
	}

	return nil
}

func (i *ipam) releaseForAllNICs(ctx context.Context, containerID, nic string, endpoint *spiderpoolv1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)

	rollback := i.getRollback(containerID)
	if len(rollback) != 0 {
		details := convertResultsToIPDetails(rollback)
		logger.Sugar().Infof("Roll back IP allocation details: %+v", details)

		if err := i.release(ctx, containerID, details); err != nil {
			return fmt.Errorf("failed to roll back the allocated IP addresses: %v", err)
		}
		i.removeRollback(containerID)
		logger.Info("Succeed to roll back")

		return nil
	}

	allocation := workloadendpointmanager.RetrieveIPAllocation(containerID, nic, endpoint)
	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}

	// Check whether an STS needs to release its currently allocated IP addresses.
	// It is discussed in https://github.com/spidernet-io/spiderpool/issues/1045
	if i.config.EnableStatefulSet && endpoint.Status.OwnerControllerType == constant.KindStatefulSet {
		clean, err := i.shouldReleaseStatefulSet(ctx, endpoint.Namespace, endpoint.Name, allocation)
		if err != nil {
			return err
		}
		if !clean {
			logger.Info("There is no need to release the IP allocation of StatefulSet")
			return nil
		}
	}

	logger.Sugar().Infof("Release IP allocation details: %+v", allocation.IPs)
	if err := i.release(ctx, allocation.ContainerID, allocation.IPs); err != nil {
		return err
	}

	logger.Info("Clear the current IP allocation")
	if err := i.endpointManager.ClearCurrentIPAllocation(ctx, containerID, endpoint); err != nil {
		return fmt.Errorf("failed to clear current IP allocation: %v", err)
	}

	logger.Info("Succeed to release")

	return nil
}

// shouldReleaseStatefulSet checks whether the StatefulSet pod need to be released, if the StatefulSet object was deleted or decreased its replicas.
// And we'll also check whether the StatefulSet pod's last ipam allocation is invalid or not,
// if we set dual stack but only get one IP allocation, we should clean up it.
func (i *ipam) shouldReleaseStatefulSet(ctx context.Context, podNamespace, podName string, allocation *spiderpoolv1.PodIPAllocation) (bool, error) {
	logger := logutils.FromContext(ctx)

	isValidStsPod, err := i.stsManager.IsValidStatefulSetPod(ctx, podNamespace, podName, constant.KindStatefulSet)
	if err != nil {
		return false, err
	}

	// StatefulSet has been deleted or scaled down.
	if !isValidStsPod {
		return true, nil
	}

	// The last allocation failed, try to clean up all allocated IP addresses
	// and re-allocate in the next time.
	for _, d := range allocation.IPs {
		if i.config.EnableIPv4 && d.IPv4 == nil ||
			i.config.EnableIPv6 && d.IPv6 == nil {
			logger.Sugar().Warnf("The Pod of StatefulSet has legacy failure allocation %+v", d)
			return true, nil
		}
	}

	// When len(allocation.IPs) == 0, no cleaning is required. Of course, this
	// situation will be handled in RetrieveIPAllocation() first.
	return false, nil
}

func (i *ipam) release(ctx context.Context, containerID string, details []spiderpoolv1.IPAllocationDetail) error {
	if len(details) == 0 {
		return nil
	}

	logger := logutils.FromContext(ctx)
	pics := GroupIPDetails(containerID, "", details)
	tickets := pics.Pools()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return fmt.Errorf("failed to queue correctly: %v", err)
	}
	defer i.ipamLimiter.ReleaseTicket(ctx, tickets...)

	errCh := make(chan error, len(pics))
	wg := sync.WaitGroup{}
	wg.Add(len(pics))

	for p, ics := range pics {
		go func(poolName string, ipAndCIDs []types.IPAndCID) {
			defer wg.Done()

			if err := i.ipPoolManager.ReleaseIP(ctx, poolName, ipAndCIDs); err != nil {
				logger.Warn(err.Error())
				errCh <- err
				return
			}
			logger.Sugar().Infof("Succeed to release IP addresses %+v from IPPool %s", ipAndCIDs, poolName)
		}(p, ics)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to release all allocated IP addresses %+v: %w", pics, utilerrors.NewAggregate(errs))
	}

	return nil
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
		log.Sugar().Infof("orphan pod dead, try to delete corresponding IPPool list that already exist")
		err := i.ipPoolManager.DeleteAllIPPools(ctx, &spiderpoolv1.SpiderIPPool{}, client.MatchingLabels{
			// this label make it sure to find orphan pod corresponding IPPool
			constant.LabelIPPoolOwnerApplication: subnetmanagercontrollers.AppLabelValue(constant.KindPod, podNS, podName),
			// TODO(Icarus9913): should we delete all interfaces auto-created IPPool in the first cmdDel?
			constant.LabelIPPoolInterface:     ifName,
			constant.LabelIPPoolReclaimIPPool: constant.True,
		})
		if nil != err {
			return err
		}
	}

	return nil
}

func (i *ipam) addRollback(containerID string, results []*AllocationResult) {
	i.rollbacks.Store(containerID, results)
}

func (i *ipam) removeRollback(containerID string) {
	i.rollbacks.Delete(containerID)
}

func (i *ipam) getRollback(containerID string) []*AllocationResult {
	v, ok := i.rollbacks.Load(containerID)
	if !ok {
		return nil
	}

	results, ok := v.([]*AllocationResult)
	if !ok {
		return nil
	}

	return results
}

func (i *ipam) Start(ctx context.Context) error {
	return i.ipamLimiter.Start(ctx)
}
