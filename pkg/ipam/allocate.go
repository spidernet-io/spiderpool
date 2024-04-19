// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/multuscniconfig"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

func (i *ipam) Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to allocate")

	pod, err := i.podManager.GetPodByName(ctx, *addArgs.PodNamespace, *addArgs.PodName, constant.UseCache)
	if err != nil {
		return nil, fmt.Errorf("failed to get Pod %s/%s: %v", *addArgs.PodNamespace, *addArgs.PodName, err)
	}
	isAlive := podmanager.IsPodAlive(pod)
	if !isAlive {
		return nil, fmt.Errorf("dead Pod %s/%s, we cannot allocate IP addresees to it", pod.Namespace, pod.Name)
	}

	podTopController, err := i.podManager.GetPodTopController(ctx, pod)
	if nil != err {
		return nil, fmt.Errorf("failed to get the top controller of the Pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
	logger.Sugar().Debugf("%s %s/%s is the top controller of the Pod", podTopController.Kind, podTopController.Namespace, podTopController.Name)

	endpointName := pod.Name
	if i.config.EnableKubevirtStaticIP && podTopController.APIVersion == kubevirtv1.SchemeGroupVersion.String() && podTopController.Kind == constant.KindKubevirtVMI {
		endpointName = podTopController.Name
	}
	endpoint, err := i.endpointManager.GetEndpointByName(ctx, pod.Namespace, endpointName, constant.UseCache)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get Endpoint %s/%s: %v", pod.Namespace, pod.Name, err)
	}
	if endpoint != nil {
		logger.Sugar().Debugf("Get Endpoint %s/%s", pod.Namespace, pod.Name)
	} else {
		logger.Debug("No Endpoint")
	}

	if (i.config.EnableStatefulSet && podTopController.APIVersion == appsv1.SchemeGroupVersion.String() && podTopController.Kind == constant.KindStatefulSet) ||
		(i.config.EnableKubevirtStaticIP && podTopController.APIVersion == kubevirtv1.SchemeGroupVersion.String() && podTopController.Kind == constant.KindKubevirtVMI) {
		logger.Sugar().Infof("Try to retrieve the IP allocation of %s", podTopController.Kind)
		addResp, err := i.retrieveStaticIPAllocation(ctx, *addArgs.IfName, pod, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the IP allocation of %s/%s/%s: %w", podTopController.Kind, podTopController.Namespace, podTopController.Name, err)
		}
		if addResp != nil {
			return addResp, nil
		}
	} else {
		logger.Debug("Try to retrieve the existing IP allocation")
		addResp, err := i.retrieveExistingIPAllocation(ctx, string(pod.UID), *addArgs.IfName, endpoint, IsMultipleNicWithNoName(pod.Annotations))
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the existing IP allocation: %w", err)
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

func (i *ipam) retrieveStaticIPAllocation(ctx context.Context, nic string, pod *corev1.Pod, endpoint *spiderpoolv2beta1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	allocation := workloadendpointmanager.RetrieveIPAllocation(string(pod.UID), nic, endpoint, true)
	if allocation == nil {
		// The first allocation or multi-NIC.
		logger.Debug("IP allocation is not found, try to allocate IP in standard mode instead of retrieving")
		return nil, nil
	}

	logger.Info("Concurrently refresh IP records of IPPools")
	if err := i.reallocateIPPoolIPRecords(ctx, string(pod.UID), endpoint); err != nil {
		return nil, fmt.Errorf("failed to reallocate IPPool IP records, error: %w", err)
	}

	logger.Info("Refresh the current IP allocation of the Endpoint")
	if err := i.endpointManager.ReallocateCurrentIPAllocation(ctx, string(pod.UID), pod.Spec.NodeName, nic, endpoint, IsMultipleNicWithNoName(pod.Annotations)); err != nil {
		return nil, fmt.Errorf("failed to refresh the current IP allocation of %s: %w", endpoint.Status.OwnerControllerType, err)
	}

	ips, routes := convert.ConvertIPDetailsToIPConfigsAndAllRoutes(endpoint.Status.Current.IPs)
	addResp := &models.IpamAddResponse{
		Ips:    ips,
		Routes: routes,
	}
	result, err := addResp.MarshalBinary()
	if nil != err {
		logger.Sugar().Infof("Succeed to retrieve the IP allocation of %s: %+v", endpoint.Status.OwnerControllerType, *addResp)
	} else {
		logger.Sugar().Infof("Succeed to retrieve the IP allocation of %s: %s", endpoint.Status.OwnerControllerType, string(result))
	}

	return addResp, nil
}

func (i *ipam) reallocateIPPoolIPRecords(ctx context.Context, uid string, endpoint *spiderpoolv2beta1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)

	namespaceKey, err := cache.MetaNamespaceKeyFunc(endpoint)
	if nil != err {
		return fmt.Errorf("failed to parse object %+v meta key", endpoint)
	}

	pius := convert.GroupIPAllocationDetails(uid, endpoint.Status.Current.IPs)
	tickets := pius.Pools()
	timeRecorder := metric.NewTimeRecorder()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return fmt.Errorf("failed to queue correctly: %v", err)
	}
	defer i.ipamLimiter.ReleaseTicket(ctx, tickets...)

	// Record the metric of queuing time for allocating.
	metric.IPAMDurationConstruct.RecordIPAMAllocationLimitDuration(ctx, timeRecorder.SinceInSeconds())

	errCh := make(chan error, len(pius))
	wg := sync.WaitGroup{}
	wg.Add(len(pius))

	for p, ius := range pius {
		go func(poolName string, ipAndUIDs []types.IPAndUID) {
			defer wg.Done()

			if err := i.ipPoolManager.UpdateAllocatedIPs(ctx, poolName, namespaceKey, ipAndUIDs); err != nil {
				logger.Warn(err.Error())
				errCh <- err
				return
			}
			logger.Sugar().Infof("Succeed to re-allocate IP addresses %+v from IPPool %s", ipAndUIDs, poolName)
		}(p, ius)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to re-allocate all allocated IP addresses %+v: %w", pius, utilerrors.NewAggregate(errs))
	}

	return nil
}

func (i *ipam) retrieveExistingIPAllocation(ctx context.Context, uid, nic string, endpoint *spiderpoolv2beta1.SpiderEndpoint, isMultipleNicWithNoName bool) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	// Create -> Delete -> Create a Pod with the same namespace and name in
	// a short time will cause some unexpected phenomena discussed in
	// https://github.com/spidernet-io/spiderpool/issues/1187.
	if endpoint != nil && endpoint.Status.Current.UID != uid {
		return nil, fmt.Errorf("currently, the IP allocation of the Pod %s/%s (UID: %s) is being recycled. You may create two Pods with the same namespace and name in a very short time", endpoint.Namespace, endpoint.Name, endpoint.Status.Current.UID)
	}

	allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic, endpoint, false)
	if allocation == nil {
		logger.Debug("Nothing retrieved to allocate")
		return nil, nil
	}

	// update Endpoint NIC name in multiple NIC with no name mode by annotation "ipam.spidernet.io/ippools"
	if isMultipleNicWithNoName {
		var err error
		allocation, err = i.endpointManager.UpdateAllocationNICName(ctx, endpoint, nic)
		if nil != err {
			return nil, fmt.Errorf("failed to update SpiderEndpoint allocation details NIC name %s, error: %v", nic, err)
		}
	}

	ips, routes := convert.ConvertIPDetailsToIPConfigsAndAllRoutes(allocation.IPs)
	addResp := &models.IpamAddResponse{
		Ips:    ips,
		Routes: routes,
	}
	result, err := addResp.MarshalBinary()
	if nil != err {
		logger.Sugar().Infof("Succeed to retrieve the IP allocation: %+v", *addResp)
	} else {
		logger.Sugar().Infof("Succeed to retrieve the IP allocation: %s", string(result))
	}

	return addResp, nil
}

func (i *ipam) allocateInStandardMode(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, endpoint *spiderpoolv2beta1.SpiderEndpoint, podController types.PodTopController) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	isMultipleNicWithNoName := IsMultipleNicWithNoName(pod.Annotations)

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

	var results []*types.AllocationResult
	defer func() {
		if err != nil && !isMultipleNicWithNoName {
			if len(results) != 0 {
				i.failure.addFailureIPs(string(pod.UID), results)
			}
			return
		}
		i.failure.rmFailureIPs(string(pod.UID))
	}()

	logger.Debug("Concurrently allocate IP addresses from all IPPool candidates")
	results, err = i.allocateIPsFromAllCandidates(ctx, toBeAllocatedSet, pod, podController)
	if err != nil {
		return nil, err
	}

	logger.Debug("Group custom routes by IP allocation results")
	if err = groupCustomRoutes(ctx, customRoutes, results); err != nil {
		return nil, fmt.Errorf("failed to group custom routes %+v: %v", customRoutes, err)
	}

	logger.Debug("Patch IP allocation results to Endpoint")
	if err = i.endpointManager.PatchIPAllocationResults(ctx, results, endpoint, pod, podController, isMultipleNicWithNoName); err != nil {
		return nil, fmt.Errorf("failed to patch IP allocation results to Endpoint: %v", err)
	}

	// sort the results in order by NIC sequence in multiple NIC with no name specified mode
	if isMultipleNicWithNoName {
		sort.Slice(results, func(i, j int) bool {
			pre, err := strconv.Atoi(*results[i].IP.Nic)
			if nil != err {
				return false
			}
			latter, err := strconv.Atoi(*results[j].IP.Nic)
			if nil != err {
				return false
			}
			return pre < latter
		})
		for index := range results {
			if *results[index].IP.Nic == strconv.Itoa(0) {
				// replace the first NIC name from "0" to "eth0"
				*results[index].IP.Nic = constant.ClusterDefaultInterfaceName

				// replace the routes NIC name from "0" to "eth0"
				for j := range results[index].Routes {
					*results[index].Routes[j].IfName = constant.ClusterDefaultInterfaceName
				}
			}
		}
	}

	resIPs, resRoutes := convert.ConvertResultsToIPConfigsAndAllRoutes(results)

	// Actually in allocate Standard Mode, we just need the current turn NIC allocation result,
	// but here are the all NICs results
	addResp := &models.IpamAddResponse{
		Ips:    resIPs,
		Routes: resRoutes,
	}
	result, err := addResp.MarshalBinary()
	if nil != err {
		logger.Sugar().Infof("Succeed to allocate: %+v", *addResp)
	} else {
		logger.Sugar().Infof("Succeed to allocate: %s", string(result))
	}

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
	if err := i.config.checkIPVersionEnable(ctx, preliminary); err != nil {
		return nil, err
	}
	for _, t := range preliminary {
		if err := i.precheckPoolCandidates(ctx, t); err != nil {
			return nil, err
		}
	}
	logger.Sugar().Infof("Prechecked IPPool candidates: %s", preliminary)

	logger.Debug("Filter out IPPool candidates")
	for _, t := range preliminary {
		if err := i.filterPoolCandidates(ctx, t, pod, podController); err != nil {
			return nil, err
		}
	}
	logger.Sugar().Infof("Filtered IPPool candidates: %s", preliminary)

	logger.Debug("Verify IPPool candidates")
	if err := i.verifyPoolCandidates(preliminary); err != nil {
		return nil, err
	}
	logger.Info("All IPPool candidates are valid")

	// sort IPPool candidates
	sortPoolCandidates(preliminary)

	return preliminary, nil
}

func (i *ipam) allocateIPsFromAllCandidates(ctx context.Context, tt ToBeAllocateds, pod *corev1.Pod, podController types.PodTopController) ([]*types.AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	tickets := tt.Pools()
	timeRecorder := metric.NewTimeRecorder()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return nil, fmt.Errorf("failed to queue correctly: %v", err)
	}
	defer i.ipamLimiter.ReleaseTicket(ctx, tickets...)

	// Record the metric of queuing time for allocating.
	metric.IPAMDurationConstruct.RecordIPAMAllocationLimitDuration(ctx, timeRecorder.SinceInSeconds())

	n := len(tt.Candidates())
	resultCh := make(chan *types.AllocationResult, n)
	errCh := make(chan error, n)
	wg := sync.WaitGroup{}
	wg.Add(n)

	doAllocate := func(candidate *PoolCandidate, nic string, cleanGateway bool) {
		defer wg.Done()

		clogger := logger.With(zap.String("AllocateHash", fmt.Sprintf("%s-%d-%v", nic, candidate.IPVersion, candidate.Pools)))
		clogger.Sugar().Debugf("Try to allocate IPv%d IP address to NIC %s from IPPools %v", candidate.IPVersion, nic, candidate.Pools)
		result, err := i.allocateIPFromCandidate(logutils.IntoContext(ctx, clogger), candidate, nic, cleanGateway, pod, podController)
		if err != nil {
			clogger.Warn(err.Error())
			errCh <- err
			return
		}

		resultCh <- result
	}

	for _, t := range tt {
		for _, c := range t.PoolCandidates {
			go doAllocate(c, t.NIC, t.CleanGateway)
		}
	}
	wg.Wait()
	close(resultCh)
	close(errCh)

	var results []*types.AllocationResult
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

	// the results are not in order by the NIC sequence right now
	return results, nil
}

func (i *ipam) allocateIPFromCandidate(ctx context.Context, c *PoolCandidate, nic string, cleanGateway bool, pod *corev1.Pod, podController types.PodTopController) (*types.AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	for _, oldRes := range i.failure.getFailureIPs(string(pod.UID)) {
		for _, ipPool := range c.PToIPPool {
			if oldRes.IP.IPPool == ipPool.Name && *oldRes.IP.Nic == nic {
				logger.Sugar().Infof("Reuse allocated IPv%d IP %s for NIC %s from IPPool %s", c.IPVersion, *oldRes.IP.Address, nic, ipPool.Name)
				oldRes.Routes = convert.ConvertSpecRoutesToOAIRoutes(nic, ipPool.Spec.Routes)
				oldRes.CleanGateway = cleanGateway
				return oldRes, nil
			}
		}
	}

	var errs []error
	var result *types.AllocationResult
	for _, pool := range c.Pools {
		ip, err := i.ipPoolManager.AllocateIP(ctx, pool, nic, pod, podController)
		if err != nil {
			logger.Sugar().Warnf("Failed to allocate IPv%d IP address to NIC %s from IPPool %s: %v", c.IPVersion, nic, pool, err)
			errs = append(errs, err)
			continue
		}

		logger.Sugar().Infof("Allocate IPv%d IP %s to NIC %s from IPPool %s", c.IPVersion, *ip.Address, nic, pool)
		result = &types.AllocationResult{
			IP:           ip,
			Routes:       convert.ConvertSpecRoutesToOAIRoutes(nic, c.PToIPPool[pool].Spec.Routes),
			CleanGateway: cleanGateway,
		}

		break
	}

	if len(errs) == len(c.Pools) {
		return nil, fmt.Errorf("failed to allocate any IPv%d IP address to NIC %s from IPPools %v: %w", c.IPVersion, nic, c.Pools, utilerrors.NewAggregate(errs))
	}

	return result, nil
}

func (i *ipam) precheckPoolCandidates(ctx context.Context, t *ToBeAllocated) error {
	logger := logutils.FromContext(ctx)

	for _, c := range t.PoolCandidates {
		if c.PToIPPool == nil {
			c.PToIPPool = PoolNameToIPPool{}
		}

		marks := map[string]bool{}
		for _, pool := range c.Pools {
			if _, ok := marks[pool]; ok {
				return fmt.Errorf("%w, duplicate IPPool %s specified for NIC %s", constant.ErrWrongInput, pool, t.NIC)
			}
			marks[pool] = true

			if _, ok := c.PToIPPool[pool]; ok {
				logger.Sugar().Debugf("Original IPPool %s has been pre-got, skip querying it again", pool)
				continue
			}

			logger.Sugar().Debugf("Get original IPPool %s", pool)
			ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, pool, constant.UseCache)
			if err != nil {
				return fmt.Errorf("failed to get original IPPool %s: %v", pool, err)
			}
			c.PToIPPool[pool] = ipPool
		}
	}

	return nil
}

func (i *ipam) filterPoolCandidates(ctx context.Context, t *ToBeAllocated, pod *corev1.Pod, podTopController types.PodTopController) error {
	logger := logutils.FromContext(ctx)

	for _, c := range t.PoolCandidates {
		cp := make([]string, len(c.Pools))
		copy(cp, c.Pools)

		var errs []error
		for j := 0; j < len(c.Pools); j++ {
			pool := c.Pools[j]
			if err := i.selectByPod(ctx, c.IPVersion, c.PToIPPool[pool], pod, podTopController, t.NIC); err != nil {
				logger.Sugar().Warnf("IPPool %s is filtered by Pod: %v", pool, err)
				errs = append(errs, err)

				delete(c.PToIPPool, pool)
				c.Pools = append((c.Pools)[:j], (c.Pools)[j+1:]...)
				j--
			}
		}

		if len(c.Pools) == 0 {
			return fmt.Errorf("%w, all IPv%d IPPools %v of %s filtered out: %v", constant.ErrNoAvailablePool, c.IPVersion, cp, t.NIC, utilerrors.NewAggregate(errs))
		}
	}

	return nil
}

func (i *ipam) selectByPod(ctx context.Context, version types.IPVersion, ipPool *spiderpoolv2beta1.SpiderIPPool, pod *corev1.Pod, podTopController types.PodTopController, nic string) error {
	if ipPool.DeletionTimestamp != nil {
		return fmt.Errorf("terminating IPPool %s", ipPool.Name)
	}

	if *ipPool.Spec.Disable {
		return fmt.Errorf("disabled IPPool %s", ipPool.Name)
	}

	if *ipPool.Spec.IPVersion != version {
		return fmt.Errorf("expect an IPv%d IPPool, but the version of the IPPool %s is IPv%d", version, ipPool.Name, *ipPool.Spec.IPVersion)
	}

	// node
	if len(ipPool.Spec.NodeName) != 0 {
		if !slices.Contains(ipPool.Spec.NodeName, pod.Spec.NodeName) {
			return fmt.Errorf("unmatched Node name of IPPool %s", ipPool.Name)
		}
	} else {
		if ipPool.Spec.NodeAffinity != nil {
			node, err := i.nodeManager.GetNodeByName(ctx, pod.Spec.NodeName, constant.UseCache)
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
	}

	// namespace
	if len(ipPool.Spec.NamespaceName) != 0 {
		if !slices.Contains(ipPool.Spec.NamespaceName, pod.Namespace) {
			return fmt.Errorf("unmatched Namespace name of IPPool %s", ipPool.Name)
		}
	} else {
		if ipPool.Spec.NamespaceAffinity != nil {
			namespace, err := i.nsManager.GetNamespaceByName(ctx, pod.Namespace, constant.UseCache)
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
	}

	// pod affinity
	if ipPool.Spec.PodAffinity != nil {
		if ippoolmanager.IsAutoCreatedIPPool(ipPool) {
			if !ippoolmanager.IsMatchAutoPoolAffinity(ipPool.Spec.PodAffinity, podTopController) {
				return fmt.Errorf("unmatched Pod annifity of auto-created IPool %s", ipPool.Name)
			}

			return nil
		}

		selector, err := metav1.LabelSelectorAsSelector(ipPool.Spec.PodAffinity)
		if err != nil {
			return err
		}
		if !selector.Matches(labels.Set(pod.Labels)) {
			return fmt.Errorf("unmatched Pod affinity of IPPool %s", ipPool.Name)
		}
	}

	// multus
	if len(ipPool.Spec.MultusName) != 0 {
		var multusNS, multusName string

		podAnno := pod.GetAnnotations()
		// the first NIC
		if nic == constant.ClusterDefaultInterfaceName || nic == strconv.Itoa(0) {
			// default net-attach-def specified in the annotations
			defaultMultusObj := podAnno[constant.MultusDefaultNetAnnot]
			if len(defaultMultusObj) == 0 {
				if i.config.MultusClusterNetwork == nil {
					return fmt.Errorf("cluster-network multus isn't set, the IPPool %v specified multusName %s unmatched", ipPool.Name, ipPool.Spec.MultusName)
				}
				defaultMultusObj = *i.config.MultusClusterNetwork
			}

			netNsName, networkName, _, err := multuscniconfig.ParsePodNetworkObjectName(defaultMultusObj)
			if nil != err {
				return fmt.Errorf("failed to parse Annotation '%s' value '%s', error: %v", constant.MultusDefaultNetAnnot, defaultMultusObj, err)
			}

			multusNS = netNsName
			if multusNS == "" {
				// Reference from Multus source codes: The CRD object of default network should only be defined in multusNamespace
				// In multus, multusNamespace serves for (clusterNetwork/defaultNetworks)
				multusNS = i.config.AgentNamespace
			}
			multusName = networkName
		} else {
			// the additional NICs must own a Multus CR object
			networkSelectionElements, err := multuscniconfig.ParsePodNetworkAnnotation(podAnno[constant.MultusNetworkAttachmentAnnot], pod.Namespace)
			if nil != err {
				return fmt.Errorf("failed to parse pod network annotation: %v", err)
			}

			isFound := false
			for idx := range networkSelectionElements {
				if len(networkSelectionElements[idx].InterfaceRequest) == 0 {
					networkSelectionElements[idx].InterfaceRequest = fmt.Sprintf("net%d", idx+1)
				}

				// We regard the NIC name was specified by the user for the previous judgement.
				// For the latter judgement(multiple NIC with no name specified mode), we just need to check whether the sequence is same with the net-attach-def resource
				if (nic == networkSelectionElements[idx].InterfaceRequest) || (nic == strconv.Itoa(idx+1)) {
					multusNS = networkSelectionElements[idx].Namespace
					multusName = networkSelectionElements[idx].Name
					isFound = true
					break
				}
			}

			// Refer from the multus-cni source codes, for annotation "k8s.v1.cni.cncf.io/networks" value without Namespace,
			// we will regard the pod Namespace as the value's namespace
			if multusNS == "" {
				multusNS = pod.ObjectMeta.Namespace
			}

			// impossible
			if !isFound {
				return fmt.Errorf("%w: no matched multus object for NIC '%s'. The multus network-attachments: %v", constant.ErrUnknown, nic, podAnno[constant.MultusNetworkAttachmentAnnot])
			}
		}

		for index := range ipPool.Spec.MultusName {
			expectedMultusName := ipPool.Spec.MultusName[index]
			if !strings.Contains(expectedMultusName, "/") {
				// for the ippool.spec.multusName property, if the user doesn't specify the net-attach-def resource namespace,
				// we'll regard it in the Spiderpool installation namespace
				expectedMultusName = fmt.Sprintf("%s/%s", i.config.AgentNamespace, expectedMultusName)
			}

			if strings.Compare(expectedMultusName, fmt.Sprintf("%s/%s", multusNS, multusName)) == 0 {
				return nil
			}
		}
		return fmt.Errorf("The spec.multusName %v in the IPPool %v used by the Pod interface %v is not matched with the multusCR %v/%v specified by the Pod.", ipPool.Spec.MultusName, ipPool.Name, nic, multusNS, multusName)
	}

	return nil
}

func (i *ipam) verifyPoolCandidates(tt ToBeAllocateds) error {

	// for _, t := range tt {
	// 	var allIPPools []*spiderpoolv2beta1.SpiderIPPool
	// 	for _, c := range t.PoolCandidates {
	// 		allIPPools = append(allIPPools, c.PToIPPool.IPPools()...)
	// 	}
	//
	// 	vlanToPools := map[types.Vlan][]string{}
	// 	for _, ipPool := range allIPPools {
	// 		vlanToPools[*ipPool.Spec.Vlan] = append(vlanToPools[*ipPool.Spec.Vlan], ipPool.Name)
	// 	}
	//
	// 	if len(vlanToPools) > 1 {
	// 		return fmt.Errorf("%w, the VLANs of the IPPools corresponding to NIC %s are not all the same: %v", constant.ErrWrongInput, t.NIC, vlanToPools)
	// 	}
	// }

	return nil
}

// sortPoolCandidates would sort IPPool candidates sequence depends on the IPPool multiple affinities.
func sortPoolCandidates(preliminary ToBeAllocateds) {
	for _, toBeAllocate := range preliminary {
		for _, poolCandidate := range (*toBeAllocate).PoolCandidates {
			// new IPPool candidate names
			poolNameList := []string{}

			// collect all IPPool resource from PoolCandidate.PToIPPool
			pools := []*spiderpoolv2beta1.SpiderIPPool{}
			for _, tmpPool := range poolCandidate.PToIPPool {
				pools = append(pools, tmpPool.DeepCopy())
			}
			// make it order with ippoolmanager.ByPoolPriority interface rules
			sort.Sort(ippoolmanager.ByPoolPriority(pools))
			for _, tmpPool := range pools {
				poolNameList = append(poolNameList, tmpPool.Name)
			}

			// set the new IPPool candidate names to PoolCandidate.Pools
			(*poolCandidate).Pools = poolNameList
		}
	}
}
