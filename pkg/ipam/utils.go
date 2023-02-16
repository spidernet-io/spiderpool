// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/singletons"
	subnetmanagercontrollers "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

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

func getCustomRoutes(pod *corev1.Pod) ([]*models.Route, error) {
	anno, ok := pod.Annotations[constant.AnnoPodRoutes]
	if !ok {
		return nil, nil
	}

	var annoPodRoutes types.AnnoPodRoutesValue
	errPrefix := fmt.Errorf("%w, invalid format of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodRoutes)
	err := json.Unmarshal([]byte(anno), &annoPodRoutes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPrefix, err)
	}

	for _, route := range annoPodRoutes {
		if err := spiderpoolip.IsRouteWithoutIPVersion(route.Dst, route.Gw); err != nil {
			return nil, fmt.Errorf("%w: %v", errPrefix, err)
		}
	}

	return convertAnnoPodRoutesToOAIRoutes(annoPodRoutes), nil
}

func groupCustomRoutes(ctx context.Context, customRoutes []*models.Route, results []*AllocationResult) error {
	if len(customRoutes) == 0 {
		return nil
	}

	for _, res := range results {
		_, ipNet, err := net.ParseCIDR(*res.IP.Address)
		if err != nil {
			return err
		}

		for i := 0; i < len(customRoutes); i++ {
			route := customRoutes[i]
			if ipNet.Contains(net.ParseIP(*route.Gw)) {
				route.IfName = res.IP.Nic
				res.Routes = append(res.Routes, route)

				customRoutes = append((customRoutes)[:i], (customRoutes)[i+1:]...)
				i--
			}
		}
	}

	if len(customRoutes) != 0 {
		logger := logutils.FromContext(ctx)
		logger.Sugar().Warnf("Invalid custom routes: %+v", customRoutes)
	}

	return nil
}

// getAutoPoolIPNumberAndSelector calculates the auto-created IPPool IP number with the given params pod and pod top controller.
// If it's an orphan pod, it will return 1.
func getAutoPoolIPNumberAndSelector(pod *corev1.Pod, podController types.PodTopController) (int, *metav1.LabelSelector, error) {
	var appReplicas int
	var podSelector *metav1.LabelSelector
	var isThirdPartyController bool

	switch podController.Kind {
	// orphan pod
	case constant.KindPod:
		return 1, &metav1.LabelSelector{MatchLabels: pod.Labels}, nil
	case constant.KindDeployment:
		deployment := podController.APP.(*appsv1.Deployment)
		appReplicas = subnetmanagercontrollers.GetAppReplicas(deployment.Spec.Replicas)
		podSelector = deployment.Spec.Selector
	case constant.KindReplicaSet:
		replicaSet := podController.APP.(*appsv1.ReplicaSet)
		podSelector = replicaSet.Spec.Selector
		appReplicas = subnetmanagercontrollers.GetAppReplicas(replicaSet.Spec.Replicas)
	case constant.KindStatefulSet:
		statefulSet := podController.APP.(*appsv1.StatefulSet)
		appReplicas = subnetmanagercontrollers.GetAppReplicas(statefulSet.Spec.Replicas)
		podSelector = statefulSet.Spec.Selector
	case constant.KindDaemonSet:
		daemonSet := podController.APP.(*appsv1.DaemonSet)
		appReplicas = int(daemonSet.Status.DesiredNumberScheduled)
		podSelector = daemonSet.Spec.Selector
	case constant.KindJob:
		job := podController.APP.(*batchv1.Job)
		appReplicas = subnetmanagercontrollers.CalculateJobPodNum(job.Spec.Parallelism, job.Spec.Completions)
		podSelector = job.Spec.Selector
	case constant.KindCronJob:
		cronJob := podController.APP.(*batchv1.CronJob)
		appReplicas = subnetmanagercontrollers.CalculateJobPodNum(cronJob.Spec.JobTemplate.Spec.Parallelism, cronJob.Spec.JobTemplate.Spec.Completions)
		podSelector = cronJob.Spec.JobTemplate.Spec.Selector
	default:
		isThirdPartyController = true
	}

	var flexibleIPNum int
	poolIPNumStr, ok := pod.Annotations[constant.AnnoSpiderSubnetPoolIPNumber]
	if ok {
		isFlexible, ipNum, err := subnetmanagercontrollers.GetPoolIPNumber(poolIPNumStr)
		if nil != err {
			return -1, nil, err
		}

		// check out negative number
		if ipNum < 0 {
			return -1, nil, fmt.Errorf("subnet '%s' value must equal or greater than 0", constant.AnnoSpiderSubnetPoolIPNumber)
		}

		// fixed IP number, just return it
		if !isFlexible {
			return ipNum, podSelector, nil
		}

		// flexible IP Number
		flexibleIPNum = ipNum
	} else {
		// third party controller only supports fixed auto-created IPPool IP number
		if isThirdPartyController {
			return -1, nil, fmt.Errorf("%s/%s/%s only supports fixed auto-created IPPool IP Number", podController.Kind, podController.Namespace, podController.Name)
		}

		// use cluster subnet default flexible IP number
		flexibleIPNum = singletons.ClusterDefaultPool.ClusterSubnetDefaultFlexibleIPNumber
	}

	// collect application replicas and custom flexible IP number
	poolIPNum := appReplicas + flexibleIPNum

	return poolIPNum, podSelector, nil
}
