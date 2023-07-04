// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	subnetmanagercontrollers "github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

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

	return convert.ConvertAnnoPodRoutesToOAIRoutes(annoPodRoutes), nil
}

// isDefaultRoute checks whether the route is default route or not
func isDefaultRoute(route models.Route) bool {
	if strings.Compare(*route.Dst, constant.IPv4AllNet) == 0 ||
		strings.Compare(*route.Dst, constant.IPv6AllNet) == 0 {
		return true
	}

	return false
}

func hasDefaultRoute(routes []*models.Route) bool {
	for index := range routes {
		if isDefaultRoute(*routes[index]) {
			return true
		}
	}

	return false
}

// inheritIPPoolRoutes will assemble the IPPool's routes and default route with gateway.
// if the IPPool's routes has default route, it will write over the gateway default route.
func inheritIPPoolRoutes(cleanGateway bool, nic string, poolGateway *string, specRoutes []spiderpoolv2beta1.Route) []*models.Route {
	poolRoutes := convert.ConvertSpecRoutesToOAIRoutes(nic, specRoutes)
	if !hasDefaultRoute(poolRoutes) {
		if poolGateway != nil {
			poolRoutes = append(poolRoutes, defaultRoute(cleanGateway, nic, *poolGateway)...)
		}
	}

	return poolRoutes
}

func defaultRoute(cleanGateway bool, nic, gateway string) []*models.Route {
	var routes []*models.Route

	// cleanGateway means we don't need default route
	if cleanGateway {
		return nil
	}

	routes = append(routes, convert.GenDefaultRoute(nic, gateway))
	return routes
}

func groupCustomRoutes(ctx context.Context, customRoutes []*models.Route, results []*types.AllocationResult) error {
	if len(customRoutes) == 0 {
		return nil
	}

	for _, res := range results {
		_, ipNet, err := net.ParseCIDR(*res.IP.Address)
		if err != nil {
			return err
		}

		// move the NIC custom routes to a new slice
		var nicCustomRoutes []*models.Route
		for i := 0; i < len(customRoutes); i++ {
			route := customRoutes[i]
			if ipNet.Contains(net.ParseIP(*route.Gw)) {
				route.IfName = res.IP.Nic
				nicCustomRoutes = append(nicCustomRoutes, route)
				customRoutes = append((customRoutes)[:i], (customRoutes)[i+1:]...)
				i--
			}
		}

		// write over the IPPool gateway default route with customRoutes default route
		for i := 0; i < len(res.Routes); i++ {
			for j := 0; j < len(nicCustomRoutes); j++ {
				if isDefaultRoute(*nicCustomRoutes[j]) && isDefaultRoute(*res.Routes[i]) {
					res.Routes[i] = nicCustomRoutes[j]
					nicCustomRoutes = append((nicCustomRoutes)[:j], (nicCustomRoutes)[j+1:]...)
					j--
				}
			}
		}
		if len(nicCustomRoutes) != 0 {
			res.Routes = append(res.Routes, nicCustomRoutes...)
		}
	}

	if len(customRoutes) != 0 {
		logger := logutils.FromContext(ctx)
		logger.Sugar().Warnf("Invalid custom routes: %+v", customRoutes)
	}

	return nil
}

// getAutoPoolIPNumber calculates the auto-created IPPool IP number with the given params pod and pod top controller.
// If it's an orphan pod, it will return 1.
func getAutoPoolIPNumber(pod *corev1.Pod, podController types.PodTopController) (int, error) {
	var appReplicas int
	var isThirdPartyController bool

	if slices.Contains(constant.K8sAPIVersions, podController.APIVersion) {
		switch podController.Kind {
		// orphan pod
		case constant.KindPod:
			return 1, nil
		case constant.KindDeployment:
			deployment := podController.APP.(*appsv1.Deployment)
			appReplicas = subnetmanagercontrollers.GetAppReplicas(deployment.Spec.Replicas)
		case constant.KindReplicaSet:
			replicaSet := podController.APP.(*appsv1.ReplicaSet)
			appReplicas = subnetmanagercontrollers.GetAppReplicas(replicaSet.Spec.Replicas)
		case constant.KindStatefulSet:
			statefulSet := podController.APP.(*appsv1.StatefulSet)
			appReplicas = subnetmanagercontrollers.GetAppReplicas(statefulSet.Spec.Replicas)
		case constant.KindDaemonSet:
			daemonSet := podController.APP.(*appsv1.DaemonSet)
			appReplicas = int(daemonSet.Status.DesiredNumberScheduled)
		case constant.KindJob:
			job := podController.APP.(*batchv1.Job)
			appReplicas = subnetmanagercontrollers.CalculateJobPodNum(job.Spec.Parallelism, job.Spec.Completions)
		case constant.KindCronJob:
			cronJob := podController.APP.(*batchv1.CronJob)
			appReplicas = subnetmanagercontrollers.CalculateJobPodNum(cronJob.Spec.JobTemplate.Spec.Parallelism, cronJob.Spec.JobTemplate.Spec.Completions)
		default:
			isThirdPartyController = true
		}
	} else {
		isThirdPartyController = true
	}

	var flexibleIPNum int
	poolIPNumStr, ok := pod.Annotations[constant.AnnoSpiderSubnetPoolIPNumber]
	if ok {
		isFlexible, ipNum, err := subnetmanagercontrollers.GetPoolIPNumber(poolIPNumStr)
		if nil != err {
			return -1, err
		}

		// check out negative number
		if ipNum < 0 {
			return -1, fmt.Errorf("subnet '%s' value must equal or greater than 0", constant.AnnoSpiderSubnetPoolIPNumber)
		}

		// fixed IP number, just return it
		if !isFlexible {
			return ipNum, nil
		}

		// flexible IP Number
		flexibleIPNum = ipNum
	} else {
		// third party controller only supports fixed auto-created IPPool IP number
		if isThirdPartyController {
			return -1, fmt.Errorf("%s/%s/%s only supports fixed auto-created IPPool IP Number", podController.Kind, podController.Namespace, podController.Name)
		}

		flexibleIPNum = 0
	}

	// collect application replicas and custom flexible IP number
	poolIPNum := appReplicas + flexibleIPNum

	return poolIPNum, nil
}

// isPoolIPsDesired checks the auto-created IPPool's IPs whether matches its AutoDesiredIPCount
func isPoolIPsDesired(pool *spiderpoolv2beta1.SpiderIPPool, desiredIPCount int) bool {
	totalIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return false
	}

	if len(totalIPs) == desiredIPCount {
		return true
	}

	return false
}
