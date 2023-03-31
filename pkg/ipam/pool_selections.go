// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	subnetmanagercontrollers "github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (i *ipam) getPoolCandidates(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, podController types.PodTopController) (ToBeAllocateds, error) {
	// If feature SpiderSubnet is enabled, select IPPool candidates through the
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

	// Select IPPools whose spec.default is true.
	t, err = i.getClusterDefaultPools(ctx, *addArgs.IfName, addArgs.CleanGateway)
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
	poolIPNum, err := getAutoPoolIPNumber(pod, podController)
	if nil != err {
		return nil, err
	}

	// This function will find the IPPool with the assembled IPPool name
	// If the application is an orphan pod and do not find any IPPool, it will return immediately to inform you to create IPPool.
	findSubnetIPPool := func(subnetName string, ipVersion types.IPVersion) (*spiderpoolv2beta1.SpiderIPPool, error) {
		poolName := subnetmanagercontrollers.SubnetPoolName(podController.Name, ipVersion, nic, podController.UID)
		var isRetried bool
		defer func() {
			if isRetried {
				metric.AutoPoolWaitedForAvailableCounts.Add(ctx, 1)
			}
		}()

		var pool *spiderpoolv2beta1.SpiderIPPool
		var err error
		for j := 1; j <= i.config.OperationRetries; j++ {
			if j > 1 {
				isRetried = true
			}
			// third-party controller applications
			if !slices.Contains(constant.K8sAPIVersions, podController.APIVersion) || !slices.Contains(constant.K8sKinds, podController.Kind) {
				pool, err = i.applyThirdControllerAutoPool(ctx, subnetName, poolName, podController, types.AutoPoolProperty{
					DesiredIPNumber: poolIPNum,
					IPVersion:       ipVersion,
					IsReclaimIPPool: false,
					IfName:          nic,
					PodSelector:     nil,
				})
			} else {
				pool, err = i.ipPoolManager.GetIPPoolByName(ctx, poolName, constant.UseCache)
			}

			if nil != err {
				if apierrors.IsNotFound(err) {
					logger.Sugar().Warnf("fetch SubnetIPPool %d times: no '%s' IPPool retrieved from SpiderSubnet '%s', wait for a second and get a retry", j, poolName, subnetName)
					time.Sleep(i.config.OperationGapDuration)
					continue
				}

				if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) || errors.Is(err, constant.ErrFreeIPsNotEnough) {
					logger.Sugar().Warnf("fetch SubnetIPPool %d times: apply auto-created IPPool conflict: %v", j, err)
					time.Sleep(i.config.OperationGapDuration)
					continue
				}

				// we should just return error directly if we meet other errors
				return nil, err
			}

			// found
			logger.Sugar().Debugf("found SpiderSubnet '%s' IPPool '%v' ", subnetName, pool)

			// TODO(Icarus9913): If shrink failed, it'll cost the whole loop
			// we fetched Auto-created IPPool but it doesn't have any IPs, just wait for a while and let the IPPool informer to allocate IPs for it
			if !isPoolIPsDesired(pool, poolIPNum) {
				logger.Sugar().Warnf("fetch SubnetIPPool %d times: retrieved IPPool '%s' but doesn't have the desiredIPNumber IPs, wait for a second and get a retry", j, poolName)
				time.Sleep(i.config.OperationGapDuration)
				continue
			}
			break
		}

		if err != nil {
			return nil, fmt.Errorf("auto-created IPPool '%s' is not available, please check it whether exists or has the desiredIPNumber IPs: %v", poolName, err)
		}
		return pool, nil
	}

	var v4PoolCandidate, v6PoolCandidate *spiderpoolv2beta1.SpiderIPPool
	var errV4, errV6 error
	var wg sync.WaitGroup

	// if enableIPv4 is off and get the specified SpiderSubnet IPv4 name, just filter it out
	if i.config.EnableIPv4 && len(subnetItem.IPv4) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			v4PoolCandidate, errV4 = findSubnetIPPool(subnetItem.IPv4[0], constant.IPv4)
			if nil != errV4 {
				return
			}
		}()
	}

	// if enableIPv6 is off and get the specified SpiderSubnet IPv6 name, just filter it out
	if i.config.EnableIPv6 && len(subnetItem.IPv6) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			v6PoolCandidate, errV6 = findSubnetIPPool(subnetItem.IPv6[0], constant.IPv6)
			if nil != errV6 {
				return
			}
		}()
	}

	wg.Wait()

	if errV4 != nil || errV6 != nil {
		return nil, multierr.Append(errV4, errV6)
	}

	if v4PoolCandidate != nil {
		logger.Sugar().Debugf("add IPv4 subnet IPPool '%s' to PoolCandidates", v4PoolCandidate.Name)
		result.PoolCandidates = append(result.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     []string{v4PoolCandidate.Name},
			PToIPPool: PoolNameToIPPool{v4PoolCandidate.Name: v4PoolCandidate},
		})
	}
	if v6PoolCandidate != nil {
		logger.Sugar().Debugf("add IPv6 subnet IPPool '%s' to PoolCandidates", v6PoolCandidate.Name)
		result.PoolCandidates = append(result.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     []string{v6PoolCandidate.Name},
			PToIPPool: PoolNameToIPPool{v6PoolCandidate.Name: v6PoolCandidate},
		})
	}

	return result, nil
}

func (i *ipam) applyThirdControllerAutoPool(ctx context.Context, subnetName, poolName string, podController types.PodTopController, autoPoolProperty types.AutoPoolProperty) (*spiderpoolv2beta1.SpiderIPPool, error) {
	tmpPool, err := i.ipPoolManager.GetIPPoolByName(ctx, poolName, constant.UseCache)
	if nil != err {
		if apierrors.IsNotFound(err) {
			// create one auto-created IPPool for third-party controller application
			tmpPool, err = i.subnetManager.ReconcileAutoIPPool(ctx, nil, subnetName, podController, autoPoolProperty)
			if nil != err {
				return nil, err
			}
			return tmpPool, nil
		}

		return nil, err
	}

	// check whether the auto IPPool need to scale its desiredIPNumber or not
	pool, err := i.subnetManager.ReconcileAutoIPPool(ctx, tmpPool, subnetName, podController, autoPoolProperty)
	if nil != err {
		return nil, err
	}

	return pool, nil
}

func getPoolFromPodAnnoPools(ctx context.Context, anno, nic string) (ToBeAllocateds, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPools)

	var annoPodIPPools types.AnnoPodIPPoolsValue
	errPrefix := fmt.Errorf("%w, invalid format of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodIPPools)
	err := json.Unmarshal([]byte(anno), &annoPodIPPools)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPrefix, err)
	}
	if len(annoPodIPPools) == 0 {
		return nil, fmt.Errorf("%w: value requires at least one item", errPrefix)
	}

	nicSet := map[string]struct{}{}
	for _, v := range annoPodIPPools {
		if v.NIC == "" {
			return nil, fmt.Errorf("%w: interface must be specified", errPrefix)
		}
		if _, ok := nicSet[v.NIC]; ok {
			return nil, fmt.Errorf("%w: duplicate interface %s", errPrefix, v.NIC)
		}
		nicSet[v.NIC] = struct{}{}
	}

	if _, ok := nicSet[nic]; !ok {
		return nil, fmt.Errorf("%w: interfaces do not contain that requested by runtime", errPrefix)
	}

	var tt ToBeAllocateds
	for _, v := range annoPodIPPools {
		t := &ToBeAllocated{
			NIC:          v.NIC,
			CleanGateway: v.CleanGateway,
		}
		if len(v.IPv4Pools) != 0 {
			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv4,
				Pools:     v.IPv4Pools,
			})
		}
		if len(v.IPv6Pools) != 0 {
			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv6,
				Pools:     v.IPv6Pools,
			})
		}
		tt = append(tt, t)
	}

	return tt, nil
}

func getPoolFromPodAnnoPool(ctx context.Context, anno, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPool)

	var annoPodIPPool types.AnnoPodIPPoolValue
	errPrefix := fmt.Errorf("%w, invalid format of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodIPPool)
	if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
		return nil, fmt.Errorf("%w: %v", errPrefix, err)
	}

	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}
	if len(annoPodIPPool.IPv4Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     annoPodIPPool.IPv4Pools,
		})
	}
	if len(annoPodIPPool.IPv6Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     annoPodIPPool.IPv6Pools,
		})
	}

	return t, nil
}

func (i *ipam) getPoolFromNS(ctx context.Context, namespace, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	ns, err := i.nsManager.GetNamespaceByName(ctx, namespace, constant.UseCache)
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

func getPoolFromNetConf(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, cleanGateway bool) *ToBeAllocated {
	if len(netConfV4Pool) == 0 && len(netConfV6Pool) == 0 {
		return nil
	}

	logger := logutils.FromContext(ctx)
	logger.Info("Use IPPools from CNI network configuration")

	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}
	if len(netConfV4Pool) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     netConfV4Pool,
		})
	}
	if len(netConfV6Pool) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     netConfV6Pool,
		})
	}

	return t
}

func (i *ipam) getClusterDefaultPools(ctx context.Context, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	ipPoolList, err := i.ipPoolManager.ListIPPools(
		ctx,
		constant.UseCache,
		client.MatchingFields{"spec.default": strconv.FormatBool(true)},
	)
	if err != nil {
		return nil, err
	}

	if len(ipPoolList.Items) == 0 {
		return nil, fmt.Errorf("%w, no pool selection rules of any type are specified", constant.ErrNoAvailablePool)
	}

	logger := logutils.FromContext(ctx)
	logger.Info("Use cluster default IPPools")

	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}

	var v4Pools, v6Pools []string
	v4PToIPPool := PoolNameToIPPool{}
	v6PToIPPool := PoolNameToIPPool{}
	for _, ipPool := range ipPoolList.Items {
		if *ipPool.Spec.IPVersion == constant.IPv4 {
			v4Pools = append(v4Pools, ipPool.Name)
			p := ipPool
			v4PToIPPool[ipPool.Name] = &p
		} else {
			v6Pools = append(v6Pools, ipPool.Name)
			p := ipPool
			v6PToIPPool[ipPool.Name] = &p
		}
	}

	if len(v4Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     v4Pools,
			PToIPPool: v4PToIPPool,
		})
	}
	if len(v6Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     v6Pools,
			PToIPPool: v6PToIPPool,
		})
	}

	return t, nil
}
