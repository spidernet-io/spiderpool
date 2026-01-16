// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"

	"github.com/spidernet-io/spiderpool/api/v1/controller/server/restapi/runtime"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
)

// Singleton
var (
	httpGetControllerStartup   = &_httpGetControllerStartup{controllerContext}
	httpGetControllerReadiness = &_httpGetControllerReadiness{controllerContext}
	httpGetControllerLiveness  = &_httpGetControllerLiveness{controllerContext}
)

type _httpGetControllerStartup struct {
	*ControllerContext
}

// Handle handles GET requests for k8s startup probe.
func (g *_httpGetControllerStartup) Handle(params runtime.GetRuntimeStartupParams) middleware.Responder {
	if !g.IsStartupProbe.Load() {
		return runtime.NewGetRuntimeStartupInternalServerError()
	}

	return runtime.NewGetRuntimeStartupOK()
}

type _httpGetControllerReadiness struct {
	*ControllerContext
}

// Handle handles GET requests for k8s readiness probe.
func (g *_httpGetControllerReadiness) Handle(params runtime.GetRuntimeReadinessParams) middleware.Responder {
	if err := openapi.WebhookHealthyCheck(g.webhookClient, g.Cfg.WebhookPort, nil); err != nil {
		logger.Sugar().Errorf("failed to check spiderpool-controller readiness probe, error: %w", err)
		return runtime.NewGetRuntimeReadinessInternalServerError()
	}

	if len(g.Leader.GetLeader()) == 0 {
		logger.Warn("failed to check spiderpool-controller readiness probe: there's no leader in the current cluster, please wait for a while")
		return runtime.NewGetRuntimeReadinessInternalServerError()
	}

	if g.Leader.IsElected() {
		if gcIPConfig.EnableGCIP && !g.GCManager.Health() {
			logger.Warn("failed to check spiderpool-controller readiness probe: the IP GC is still not ready, please wait for a while")
			return runtime.NewGetRuntimeReadinessInternalServerError()
		}
	}

	return runtime.NewGetRuntimeReadinessOK()
}

type _httpGetControllerLiveness struct {
	*ControllerContext
}

// Handle handles GET requests for k8s liveness probe.
func (g *_httpGetControllerLiveness) Handle(params runtime.GetRuntimeLivenessParams) middleware.Responder {
	if err := openapi.WebhookHealthyCheck(g.webhookClient, g.Cfg.WebhookPort, nil); err != nil {
		logger.Sugar().Errorf("failed to check spiderpool controller liveness probe, error: %w", err)
		return runtime.NewGetRuntimeLivenessInternalServerError()
	}

	return runtime.NewGetRuntimeLivenessOK()
}
