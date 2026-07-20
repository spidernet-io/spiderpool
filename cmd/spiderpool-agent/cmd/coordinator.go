// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/go-openapi/runtime/middleware"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

const coordinatorCNICommandHeader = "X-Spiderpool-CNI-Command"

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
	coord := coordList.Items[0]

	if coord.Status.Phase != coordinatormanager.Synced {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("spidercoordinator: %s no ready", coord.Name)))
	}

	var err error
	isDel := strings.EqualFold(params.HTTPRequest.Header.Get(coordinatorCNICommandHeader), "DEL")
	logger := logutils.Logger.Named("CoordinatorConfig").With(
		zap.String("PodNamespace", params.GetCoordinatorConfig.PodNamespace),
		zap.String("PodName", params.GetCoordinatorConfig.PodName),
	)

	var pod *corev1.Pod
	pod, err = podClient.GetPodByName(ctx, params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName, constant.UseCache)
	if err != nil {
		if !isDel {
			return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(fmt.Sprintf("failed to get pod %s/%s: %v", params.GetCoordinatorConfig.PodNamespace, params.GetCoordinatorConfig.PodName, err)))
		}

		if apierrors.IsNotFound(err) {
			logger.Warn("Pod no longer exists; use global SpiderCoordinator config for DEL", zap.Error(err))
		} else {
			logger.Warn("failed to get Pod; Pod data is not required for DEL, use global SpiderCoordinator config", zap.Error(err))
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

	var vethLinkAddress string
	if coord.Spec.VethLinkAddress != nil {
		vethLinkAddress = *coord.Spec.VethLinkAddress
	}
	var vethMTU int64
	if coord.Spec.VethMTU != nil {
		vethMTU = int64(*coord.Spec.VethMTU)
	}

	if !isDel {
		defaultRouteNic, ok := pod.Annotations[constant.AnnoDefaultRouteInterface]
		if ok {
			nic = defaultRouteNic
		}
	}

	config := &models.CoordinatorConfig{
		Mode:               coord.Spec.Mode,
		OverlayPodCIDR:     coord.Status.OverlayPodCIDR,
		ServiceCIDR:        coord.Status.ServiceCIDR,
		HijackCIDR:         coord.Spec.HijackCIDR,
		PolicyRoutes:       convertCoordinatorPolicyRoutes(coord.Spec.PolicyRoutes),
		PodMACPrefix:       prefix,
		TunePodRoutes:      coord.Spec.TunePodRoutes,
		PodDefaultRouteNIC: nic,
		VethLinkAddress:    vethLinkAddress,
		VethMTU:            vethMTU,
		HostRuleTable:      int64(*coord.Spec.HostRuleTable),
		PodRPFilter:        int64(*coord.Spec.PodRPFilter),
		TxQueueLen:         int64(*coord.Spec.TxQueueLen),
	}

	if config.OverlayPodCIDR == nil {
		config.OverlayPodCIDR = []string{}
	}
	if config.ServiceCIDR == nil {
		config.OverlayPodCIDR = []string{}
	}

	return daemonset.NewGetCoordinatorConfigOK().WithPayload(config)
}

func convertCoordinatorPolicyRoutes(routes []spiderpoolv2beta1.Route) []*models.CoordinatorRoute {
	if len(routes) == 0 {
		return nil
	}

	result := make([]*models.CoordinatorRoute, 0, len(routes))
	for idx := range routes {
		result = append(result, &models.CoordinatorRoute{
			Dst: routes[idx].Dst,
			Gw:  routes[idx].Gw,
		})
	}
	return result
}
