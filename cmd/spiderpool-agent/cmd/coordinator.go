// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/go-openapi/runtime/middleware"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var unixGetCoordinatorConfig = &_unixGetCoordinatorConfig{}

type _unixGetCoordinatorConfig struct{}

// Handle handles Get requests for /coordinator/config.
func (g *_unixGetCoordinatorConfig) Handle(params daemonset.GetCoordinatorConfigParams) middleware.Responder {
	ctx := params.HTTPRequest.Context()
	crdClient := agentContext.CRDManager.GetClient()
	podClient := agentContext.PodManager
	epClient := agentContext.EndpointManager
	kubevirtMgr := agentContext.KubevirtManager

	var coordList spiderpoolv2beta1.SpiderCoordinatorList
	if err := crdClient.List(ctx, &coordList); err != nil {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(err.Error()))
	}

	if len(coordList.Items) == 0 {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error("coordinator config not found"))
	}
	coord := coordList.Items[0]

	if coord.Status.Phase != coordinatormanager.Synced {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("spidercoordinator: %s no ready", coord.Name)))
	}

	var err error
	var spNics []string
	var se *spiderpoolv2beta1.SpiderEndpoint

	var pod *corev1.Pod
	pod, err = podClient.GetPodByName(ctx, params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName, constant.UseCache)
	if err != nil {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("failed to get pod %s/%s", params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName)))
	}

	isVMPod := false
	// kubevirt vm pod corresponding SpiderEndpoint uses kubevirt VM/VMI name
	endpointName := params.GetCoordinatorConfig.PodName
	ownerReference := metav1.GetControllerOf(pod)
	if ownerReference != nil && agentContext.Cfg.EnableKubevirtStaticIP && ownerReference.APIVersion == kubevirtv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindKubevirtVMI {
		endpointName = ownerReference.Name
		isVMPod = true
	}

	// get spiderendpoint
	se, err = epClient.GetEndpointByName(ctx, params.GetCoordinatorConfig.PodNamespace, endpointName, constant.UseCache)
	if err != nil && !apierrors.IsNotFound(err) {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("failed to get spiderendpoint %s/%s", params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName)))
	}

	// cancel IP conflict detection for the kubevirt vm live migration new pod
	detectIPConflict := *coord.Spec.DetectIPConflict
	if detectIPConflict && isVMPod {
		// the live migration new pod has the annotation "kubevirt.io/migrationJobName"
		// we just only cancel IP conflict detection for the live migration new pod.
		podAnnos := pod.GetAnnotations()
		vmimName, ok := podAnnos[kubevirtv1.MigrationJobNameAnnotation]
		if ok {
			_, err := kubevirtMgr.GetVMIMByName(ctx, pod.Namespace, vmimName, false)
			if nil != err {
				if apierrors.IsNotFound(err) {
					logger.Sugar().Warnf("no kubevirt vm pod '%s/%s' corresponding VirtualMachineInstanceMigration '%s/%s' found, still execute IP conflict detection",
						pod.Namespace, pod.Name, pod.Namespace, vmimName)
				} else {
					return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("failed to get kubevirt vm pod '%s/%s' corresponding VirtualMachineInstanceMigration '%s/%s', error: %v",
						pod.Namespace, pod.Name, pod.Namespace, vmimName, err)))
				}
			} else {
				// cancel IP conflict detection because there's a moment the old vm pod still running during the vm live migration phase
				logger.Sugar().Infof("cancel IP conflict detection for live migration new pod '%s/%s'", pod.Namespace, pod.Name)
				detectIPConflict = false
			}
		}
	}

	if se != nil {
		for _, spip := range se.Status.Current.IPs {
			spNics = append(spNics, spip.NIC)
		}
	}

	var prefix string
	if coord.Spec.PodMACPrefix != nil {
		prefix = *coord.Spec.PodMACPrefix
	}

	var nic string
	if coord.Spec.PodDefaultRouteNIC != nil {
		nic = *coord.Spec.PodDefaultRouteNIC
	}

	defaultRouteNic, ok := pod.Annotations[constant.AnnoDefaultRouteInterface]
	if ok {
		nic = defaultRouteNic
	}

	config := &models.CoordinatorConfig{
		Mode:               coord.Spec.Mode,
		OverlayPodCIDR:     coord.Status.OverlayPodCIDR,
		ServiceCIDR:        coord.Status.ServiceCIDR,
		HijackCIDR:         coord.Spec.HijackCIDR,
		PodMACPrefix:       prefix,
		TunePodRoutes:      coord.Spec.TunePodRoutes,
		PodDefaultRouteNIC: nic,
		HostRuleTable:      int64(*coord.Spec.HostRuleTable),
		HostRPFilter:       int64(*coord.Spec.HostRPFilter),
		DetectGateway:      *coord.Spec.DetectGateway,
		DetectIPConflict:   detectIPConflict,
		PodNICs:            spNics,
	}

	if config.OverlayPodCIDR == nil {
		config.OverlayPodCIDR = []string{}
	}
	if config.ServiceCIDR == nil {
		config.OverlayPodCIDR = []string{}
	}

	return daemonset.NewGetCoordinatorConfigOK().WithPayload(config)
}
