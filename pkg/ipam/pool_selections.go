// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
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
	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (i *ipam) getPoolCandidates(ctx context.Context, addArgs *models.IpamAddArgs, pod *corev1.Pod, podController types.PodTopController) (ToBeAllocateds, error) {
	// If feature SpiderSubnet is enabled, select IPPool candidates through the
	// Pod annotations "ipam.spidernet.io/subnet" or "ipam.spidernet.io/subnets". (expect orphan Pod controller)
	if i.config.EnableSpiderSubnet {
		if i.config.EnableAutoPoolForApplication {
			fromSubnet, err := i.getPoolFromSubnetAnno(ctx, pod, *addArgs.IfName, addArgs.CleanGateway, podController)
			if nil != err {
				return nil, fmt.Errorf("failed to get IPPool candidates from Subnet: %w", err)
			}
			if fromSubnet != nil {
				return ToBeAllocateds{fromSubnet}, nil
			}
		} else {
			hasSubnetsAnnotation := applicationinformers.HasSubnetsAnnotation(pod.Annotations)
			if hasSubnetsAnnotation {
				return nil, fmt.Errorf("it's invalid to use '%s' or '%s' annotation when Auto-Pool feature is disabled", constant.AnnoSpiderSubnets, constant.AnnoSpiderSubnet)
			}
		}
	}

	// Select IPPool candidates through the Pod annotation "ipam.spidernet.io/ippools".
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		return i.getPoolFromPodAnnoPools(ctx, anno, *addArgs.IfName)
	}

	// Select IPPool candidates through the Pod annotation "ipam.spidernet.io/ippool".
	if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		t, err := i.getPoolFromPodAnnoPool(ctx, anno, *addArgs.IfName, addArgs.CleanGateway)
		if err != nil {
			return nil, err
		}
		return ToBeAllocateds{t}, nil
	}

	// Select IPPool candidates through the Namespace annotations
	// "ipam.spidernet.io/default-ipv4-ippool" and "ipam.spidernet.io/default-ipv6-ippool".
	t, err := i.getPoolFromNS(ctx, pod.Namespace, *addArgs.IfName, addArgs.CleanGateway)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return ToBeAllocateds{t}, nil
	}

	// Select IPPool candidates through CNI network configuration.
	t, err = i.getPoolFromNetConf(ctx, *addArgs.IfName, addArgs.DefaultIPV4IPPool, addArgs.DefaultIPV6IPPool, addArgs.CleanGateway)
	if nil != err {
		return nil, err
	}
	if t != nil {
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
	subnetAnnoConfig, err := applicationinformers.GetSubnetAnnoConfig(pod.Annotations, logger)
	if nil != err {
		return nil, err
	}

	// default IPPool mode
	if applicationinformers.IsDefaultIPPoolMode(subnetAnnoConfig) {
		return nil, nil
	}
	// The SpiderSubnet feature doesn't support orphan Pod.
	if podController.APIVersion == corev1.SchemeGroupVersion.String() && podController.Kind == constant.KindPod {
		return nil, fmt.Errorf("SpiderSubnet feature doesn't support no-controller pod")
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

	result := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}

	// This only serves for third party controller application, because we'll create or scale the auto-created IPPool here.
	// For those kubernetes applications(such as deployment and replicaset), the spiderpool-controller will create or scale the auto-created IPPool asynchronously.
	poolIPNum, err := getAutoPoolIPNumber(pod, podController)
	if nil != err {
		return nil, err
	}

	var v4PoolCandidate, v6PoolCandidate *spiderpoolv2beta1.SpiderIPPool
	var errV4, errV6 error
	var wg sync.WaitGroup

	// if enableIPv4 is off and get the specified SpiderSubnet IPv4 name, just filter it out
	if i.config.EnableIPv4 && len(subnetItem.IPv4) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if !slices.Contains(constant.K8sAPIVersions, podController.APIVersion) || !slices.Contains(constant.K8sKinds, podController.Kind) {
				v4PoolCandidate, errV4 = i.applyThirdControllerAutoPool(ctx, subnetItem.IPv4[0], podController, types.AutoPoolProperty{
					DesiredIPNumber:     poolIPNum,
					IPVersion:           constant.IPv4,
					IsReclaimIPPool:     subnetAnnoConfig.ReclaimIPPool,
					IfName:              nic,
					AnnoPoolIPNumberVal: strconv.Itoa(poolIPNum),
				})
			} else {
				v4PoolCandidate, errV4 = i.findAppAutoPool(ctx, subnetItem.IPv4[0], nic, constant.LabelValueIPVersionV4, poolIPNum, podController)
			}

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

			if !slices.Contains(constant.K8sAPIVersions, podController.APIVersion) || !slices.Contains(constant.K8sKinds, podController.Kind) {
				v6PoolCandidate, errV6 = i.applyThirdControllerAutoPool(ctx, subnetItem.IPv6[0], podController, types.AutoPoolProperty{
					DesiredIPNumber:     poolIPNum,
					IPVersion:           constant.IPv6,
					IsReclaimIPPool:     subnetAnnoConfig.ReclaimIPPool,
					IfName:              nic,
					AnnoPoolIPNumberVal: strconv.Itoa(poolIPNum),
				})
			} else {
				v6PoolCandidate, errV6 = i.findAppAutoPool(ctx, subnetItem.IPv6[0], nic, constant.LabelValueIPVersionV6, poolIPNum, podController)
			}

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

// findAppAutoPool only fetches kubernetes basic controller(like Deployment, StatefulSet etc...) corresponding auto-created IPPools.
func (i *ipam) findAppAutoPool(ctx context.Context, subnetName, ifName, labelIPPoolIPVersionValue string, desiredIPNumber int, podController types.PodTopController) (*spiderpoolv2beta1.SpiderIPPool, error) {
	log := logutils.FromContext(ctx)

	var pool *spiderpoolv2beta1.SpiderIPPool
	matchLabels := client.MatchingLabels{
		constant.LabelIPPoolOwnerSpiderSubnet:         subnetName,
		constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(podController.APIVersion),
		constant.LabelIPPoolOwnerApplicationKind:      podController.Kind,
		constant.LabelIPPoolOwnerApplicationNamespace: podController.Namespace,
		constant.LabelIPPoolOwnerApplicationName:      podController.Name,
		constant.LabelIPPoolOwnerApplicationUID:       string(podController.UID),
		constant.LabelIPPoolInterface:                 ifName,
		constant.LabelIPPoolIPVersion:                 labelIPPoolIPVersionValue,
	}
	for j := 1; j <= i.config.OperationRetries; j++ {
		poolList, err := i.ipPoolManager.ListIPPools(ctx, constant.UseCache, matchLabels)
		if nil != err {
			return nil, fmt.Errorf("failed to get auto-created IPPoolList with labels '%v', error: %w", matchLabels, err)
		}

		if len(poolList.Items) == 0 {
			log.Sugar().Warnf("fetch SubnetIPPool %d times: no IPPool retrieved from SpiderSubnet '%s' with matchLabel '%v', wait for a second and get a retry",
				j, subnetName, matchLabels)
			time.Sleep(i.config.OperationGapDuration)
		} else if len(poolList.Items) == 1 {
			pool = poolList.Items[0].DeepCopy()
			log.Sugar().Debugf("found SpiderSubnet '%s' IPPool '%s' with matchLabel '%v'", subnetName, pool.Name, matchLabels)

			// we fetched Auto-created IPPool but it doesn't have any IPs, just wait for a while and let the IPPool informer to allocate IPs for it
			if !isPoolIPsDesired(pool, desiredIPNumber) {
				log.Sugar().Warnf("fetch SubnetIPPool %d times: retrieved IPPool '%s' but doesn't have the desiredIPNumber IPs, wait for a second and get a retry", j, pool.Name)
				time.Sleep(i.config.OperationGapDuration)
				continue
			}
			break
		} else {
			return nil, fmt.Errorf("it's invalid for '%s/%s/%s' corresponding SpiderSubnet '%s' owns multiple matchLables '%v' corresponding IPPools '%v' for one specify application",
				podController.Kind, podController.Namespace, podController.Name, subnetName, matchLabels, poolList.Items)
		}
	}
	if pool == nil {
		return nil, fmt.Errorf("no matching auto-created IPPool candidate with matchLables '%v'", matchLabels)
	}

	return pool, nil
}

// applyThirdControllerAutoPool will fetch or reconcile third-party controller corresponding auto-created IPPools,
// and the kubernetes basic controller like Deployment,StatefulSet etc... We'll reconcile their auto-created IPPools in spiderpool-controller component.
func (i *ipam) applyThirdControllerAutoPool(ctx context.Context, subnetName string, podController types.PodTopController, autoPoolProperty types.AutoPoolProperty) (*spiderpoolv2beta1.SpiderIPPool, error) {
	log := logutils.FromContext(ctx)

	var pool *spiderpoolv2beta1.SpiderIPPool
	matchLabels := client.MatchingLabels{
		constant.LabelIPPoolOwnerSpiderSubnet:         subnetName,
		constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(podController.APIVersion),
		constant.LabelIPPoolOwnerApplicationKind:      podController.Kind,
		constant.LabelIPPoolOwnerApplicationNamespace: podController.Namespace,
		constant.LabelIPPoolOwnerApplicationName:      podController.Name,
		constant.LabelIPPoolIPVersion:                 applicationinformers.AutoPoolIPVersionLabelValue(autoPoolProperty.IPVersion),
		constant.LabelIPPoolInterface:                 autoPoolProperty.IfName,
	}
	for j := 1; j <= i.config.OperationRetries; j++ {
		poolList, err := i.ipPoolManager.ListIPPools(ctx, constant.UseCache, matchLabels)
		if nil != err {
			return nil, fmt.Errorf("failed to get third-party auto-created IPPoolList with matchLabels %v, error: %w", matchLabels, err)
		}

		if len(poolList.Items) == 0 {
			pool = nil
		} else {
			for k := range poolList.Items {
				labels := poolList.Items[k].GetLabels()
				if labels[constant.LabelIPPoolReclaimIPPool] == constant.True && labels[constant.LabelIPPoolOwnerApplicationUID] != string(podController.UID) {
					log.Sugar().Debugf("found the previous same app auto-created IPPool %s", poolList.Items[k].Name)
					continue
				}

				pool = poolList.Items[k].DeepCopy()
				log.Sugar().Infof("found reuse app auto-created IPPool %s", pool.Name)
				break
			}
		}

		pool, err = i.subnetManager.ReconcileAutoIPPool(ctx, pool, subnetName, podController, autoPoolProperty)
		if nil != err {
			if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) {
				log.Sugar().Warnf("fetch SubnetIPPool %d times: apply auto-created IPPool conflict: %v", j, err)
				time.Sleep(i.config.OperationGapDuration)
				continue
			}
			return nil, fmt.Errorf("failed to check SpiderSubnet %s third-party controller auto-created IPPool whether need to be scaled: %w", subnetName, err)
		}
		break
	}
	if pool == nil {
		return nil, fmt.Errorf("no matching third-party controller auto-created IPPool candidate with matchLables '%v'", matchLabels)
	}
	return pool, nil
}

func (i *ipam) getPoolFromPodAnnoPools(ctx context.Context, anno, currentNIC string) (ToBeAllocateds, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPools)

	var annoPodIPPools types.AnnoPodIPPoolsValue
	errPrefix := fmt.Errorf("%w, invalid format of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodIPPools)
	err := json.Unmarshal([]byte(anno), &annoPodIPPools)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPrefix, err)
	}

	for index := range annoPodIPPools {
		if len(annoPodIPPools[index].IPv4Pools) != 0 {
			newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, annoPodIPPools[index].IPv4Pools, constant.IPv4)
			if nil != err {
				return nil, err
			}
			if hasWildcard {
				annoPodIPPools[index].IPv4Pools = newPoolNames
			}
		}
		if len(annoPodIPPools[index].IPv6Pools) != 0 {
			newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, annoPodIPPools[index].IPv6Pools, constant.IPv6)
			if nil != err {
				return nil, err
			}
			if hasWildcard {
				annoPodIPPools[index].IPv6Pools = newPoolNames
			}
		}
	}

	// validate and mutate the IPPools annotation value
	err = validateAndMutateMultipleNICAnnotations(annoPodIPPools, currentNIC)
	if nil != err {
		return nil, fmt.Errorf("%w: %w", errPrefix, err)
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

func (i *ipam) getPoolFromPodAnnoPool(ctx context.Context, anno, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPool)

	var annoPodIPPool types.AnnoPodIPPoolValue
	errPrefix := fmt.Errorf("%w, invalid format of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodIPPool)
	if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
		return nil, fmt.Errorf("%w: %w", errPrefix, err)
	}

	// check IPv4 PoolName wildcard
	if len(annoPodIPPool.IPv4Pools) != 0 {
		newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, annoPodIPPool.IPv4Pools, constant.IPv4)
		if nil != err {
			return nil, err
		}
		// overwrite the annoPodIPPool
		if hasWildcard {
			annoPodIPPool.IPv4Pools = newPoolNames
		}
	}
	// check IPv6 PoolName wildcard
	if len(annoPodIPPool.IPv6Pools) != 0 {
		newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, annoPodIPPool.IPv6Pools, constant.IPv6)
		if nil != err {
			return nil, err
		}
		// overwrite the annoPodIPPool
		if hasWildcard {
			annoPodIPPool.IPv6Pools = newPoolNames
		}
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

	if len(nsDefaultV4Pools) != 0 {
		newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, nsDefaultV4Pools, constant.IPv4)
		if nil != err {
			return nil, err
		}
		if hasWildcard {
			nsDefaultV4Pools = newPoolNames
		}
	}
	if len(nsDefaultV6Pools) != 0 {
		newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, nsDefaultV6Pools, constant.IPv6)
		if nil != err {
			return nil, err
		}
		if hasWildcard {
			nsDefaultV6Pools = newPoolNames
		}
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

func (i *ipam) getPoolFromNetConf(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, cleanGateway bool) (*ToBeAllocated, error) {
	if len(netConfV4Pool) == 0 && len(netConfV6Pool) == 0 {
		return nil, nil
	}
	if len(netConfV4Pool) != 0 {
		newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, netConfV4Pool, constant.IPv4)
		if nil != err {
			return nil, err
		}
		if hasWildcard {
			netConfV4Pool = newPoolNames
		}
	}
	if len(netConfV6Pool) != 0 {
		newPoolNames, hasWildcard, err := i.ipPoolManager.ParseWildcardPoolNameList(ctx, netConfV6Pool, constant.IPv6)
		if nil != err {
			return nil, err
		}
		if hasWildcard {
			netConfV6Pool = newPoolNames
		}
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

	return t, nil
}

func (i *ipam) getClusterDefaultPools(ctx context.Context, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	ipPoolList, err := i.ipPoolManager.ListIPPools(
		ctx,
		constant.UseCache,
		client.MatchingFields{constant.SpecDefaultField: strconv.FormatBool(true)},
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
