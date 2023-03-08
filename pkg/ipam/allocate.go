// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

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
		addResp, err := i.retrieveMultiNICIPAllocation(ctx, string(pod.UID), *addArgs.IfName, endpoint)
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

	logger.Info("Concurrently refresh IP records of IPPools")
	if err := i.reallocateIPPoolIPRecords(ctx, string(pod.UID), endpoint); err != nil {
		return nil, err
	}

	logger.Info("Refresh the current IP allocation of the Endpoint")
	if err := i.endpointManager.ReallocateCurrentIPAllocation(ctx, containerID, string(pod.UID), pod.Spec.NodeName, endpoint); err != nil {
		return nil, fmt.Errorf("failed to update the current IP allocation of StatefulSet: %w", err)
	}

	ips, routes := convert.ConvertIPDetailsToIPConfigsAndAllRoutes(endpoint.Status.Current.IPs)
	addResp := &models.IpamAddResponse{
		Ips:    ips,
		Routes: routes,
	}
	logger.Sugar().Infof("Succeed to retrieve the IP allocation of StatefulSet: %+v", *addResp)

	return addResp, nil
}

func (i *ipam) reallocateIPPoolIPRecords(ctx context.Context, uid string, endpoint *spiderpoolv1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)

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

			if err := i.ipPoolManager.UpdateAllocatedIPs(ctx, poolName, ipAndUIDs); err != nil {
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

func (i *ipam) retrieveMultiNICIPAllocation(ctx context.Context, uid, nic string, endpoint *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)

	allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic, endpoint)
	if allocation == nil {
		logger.Debug("Nothing retrieved to allocate")
		return nil, nil
	}

	ips, routes := convert.ConvertIPDetailsToIPConfigsAndAllRoutes(allocation.IPs)
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

	var results []*types.AllocationResult
	defer func() {
		if err != nil {
			if len(results) != 0 {
				i.cache.addFailureIPs(string(pod.UID), results)
			}
			return
		}
		i.cache.rmFailureIPs(string(pod.UID))
	}()

	logger.Debug("Concurrently allocate IP addresses from all IPPool candidates")
	results, err = i.allocateIPsFromAllCandidates(ctx, toBeAllocatedSet, pod)
	if err != nil {
		return nil, err
	}

	logger.Debug("Group custom routes by IP allocation results")
	if err = groupCustomRoutes(ctx, customRoutes, results); err != nil {
		return nil, fmt.Errorf("failed to group custom routes %+v: %v", customRoutes, err)
	}

	logger.Debug("Patch IP allocation results to Endpoint")
	if err = i.endpointManager.PatchIPAllocationResults(ctx, *addArgs.ContainerID, results, endpoint, pod, podController); err != nil {
		return nil, fmt.Errorf("failed to patch IP allocation results to Endpoint: %v", err)
	}

	resIPs, resRoutes := convert.ConvertResultsToIPConfigsAndAllRoutes(results)
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
		if err := i.filterPoolCandidates(ctx, t, pod); err != nil {
			return nil, err
		}
	}
	logger.Sugar().Infof("Filtered IPPool candidates: %s", preliminary)

	logger.Debug("Verify IPPool candidates")
	if err := i.verifyPoolCandidates(preliminary); err != nil {
		return nil, err
	}
	logger.Info("All IPPool candidates are valid")

	return preliminary, nil
}

func (i *ipam) allocateIPsFromAllCandidates(ctx context.Context, tt ToBeAllocateds, pod *corev1.Pod) ([]*types.AllocationResult, error) {
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
		result, err := i.allocateIPFromCandidate(logutils.IntoContext(ctx, clogger), candidate, nic, cleanGateway, pod)
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

	return results, nil
}

func (i *ipam) allocateIPFromCandidate(ctx context.Context, c *PoolCandidate, nic string, cleanGateway bool, pod *corev1.Pod) (*types.AllocationResult, error) {
	logger := logutils.FromContext(ctx)

	for _, oldRes := range i.cache.getFailureIPs(string(pod.UID)) {
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
		ip, err := i.ipPoolManager.AllocateIP(ctx, pool, nic, pod)
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

	// TODO(iiiceoo): Use gorouting.
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
			ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, pool)
			if err != nil {
				return fmt.Errorf("failed to get original IPPool %s: %v", pool, err)
			}
			c.PToIPPool[pool] = ipPool
		}
	}

	return nil
}

func (i *ipam) filterPoolCandidates(ctx context.Context, t *ToBeAllocated, pod *corev1.Pod) error {
	logger := logutils.FromContext(ctx)

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
