// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	corev1 "k8s.io/api/core/v1"
)

var unixGetCoordinatorConfig = &_unixGetCoordinatorConfig{}

type _unixGetCoordinatorConfig struct{}

// Handle handles Get requests for /coordinator/config.
func (g *_unixGetCoordinatorConfig) Handle(params daemonset.GetCoordinatorConfigParams) middleware.Responder {
	ctx := params.HTTPRequest.Context()
	crdClient := agentContext.CRDManager.GetClient()
	podClient := agentContext.PodManager

	var coordList spiderpoolv2beta1.SpiderCoordinatorList
	if err := crdClient.List(ctx, &coordList); err != nil {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(err.Error()))
	}

	if len(coordList.Items) == 0 {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error("coordinator config not found"))
	}

	var pod *corev1.Pod
	var err error
	pod, err = podClient.GetPodByName(ctx, params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName, constant.UseCache)
	if err != nil {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("failed to get coordinator config: pod %s/%s not found", params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName)))
	}

	coord := coordList.Items[0]
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
		DetectIPConflict:   *coord.Spec.DetectIPConflict,
	}

	return daemonset.NewGetCoordinatorConfigOK().WithPayload(config)
}
