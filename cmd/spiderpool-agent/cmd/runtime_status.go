// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/runtime"
)

// Singleton
var (
	httpGetAgentStartup   = &_httpGetAgentStartup{agentContext}
	httpGetAgentReadiness = &_httpGetAgentReadiness{}
	httpGetAgentLiveness  = &_httpGetAgentLiveness{}
)

type _httpGetAgentStartup struct {
	*AgentContext
}

// Handle handles GET requests for k8s startup probe.
func (g *_httpGetAgentStartup) Handle(params runtime.GetRuntimeStartupParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	if g.IsStartupProbe.Load() {
		return runtime.NewGetRuntimeStartupOK()
	}

	return runtime.NewGetRuntimeStartupInternalServerError()
}

type _httpGetAgentReadiness struct{}

// Handle handles GET requests for k8s readiness probe.
func (g *_httpGetAgentReadiness) Handle(params runtime.GetRuntimeReadinessParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return runtime.NewGetRuntimeReadinessOK()
}

type _httpGetAgentLiveness struct{}

// Handle handles GET requests for k8s liveness probe.
func (g *_httpGetAgentLiveness) Handle(params runtime.GetRuntimeLivenessParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return runtime.NewGetRuntimeLivenessOK()
}
