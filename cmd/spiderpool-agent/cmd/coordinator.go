// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var unixGetCoordinatorConfig = &_unixGetCoordinatorConfig{}

type _unixGetCoordinatorConfig struct{}

// Handle handles Get requests for /coordinator/config.
func (g *_unixGetCoordinatorConfig) Handle(params daemonset.GetCoordinatorConfigParams) middleware.Responder {
	ctx := params.HTTPRequest.Context()
	client := agentContext.CRDManager.GetClient()

	var coordList spiderpoolv2beta1.SpiderCoordinatorList
	if err := client.List(ctx, &coordList); err != nil {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error(err.Error()))
	}

	if len(coordList.Items) == 0 {
		return daemonset.NewGetCoordinatorConfigFailure().WithPayload(models.Error("coordinator config not found"))
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

	config := &models.CoordinatorConfig{
		TuneMode:           coord.Spec.TuneMode,
		PodCIDR:            coord.Status.PodCIDR,
		ServiceCIDR:        coord.Status.ServiceCIDR,
		ExtraCIDR:          coord.Spec.ExtraCIDR,
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
