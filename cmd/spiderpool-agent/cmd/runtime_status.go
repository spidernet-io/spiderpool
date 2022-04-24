// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-agent/server/restapi/runtime"
	runtimeops "github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-agent/server/restapi/runtime"
)

type getAgentStartup struct{}

// NewGetAgentRuntimeStartupHandler handles requests for agent runtime startup probe.
func NewGetAgentRuntimeStartupHandler() runtimeops.GetRuntimeStartupHandler {
	return &getAgentStartup{}
}

// Handle handles GET requests for /runtime/startup .
func (g *getAgentStartup) Handle(params runtimeops.GetRuntimeStartupParams) middleware.Responder {
	// TODO: return the http status code with params.

	return runtime.NewGetRuntimeStartupOK()
}

type getAgentReadiness struct{}

// NewGetAgentRuntimeReadinessHandler handles requests for agent runtime readiness probe.
func NewGetAgentRuntimeReadinessHandler() runtimeops.GetRuntimeReadinessHandler {
	return &getAgentReadiness{}
}

// Handle handles GET requests for /runtime/readiness .
func (g *getAgentReadiness) Handle(params runtimeops.GetRuntimeReadinessParams) middleware.Responder {
	// TODO: return the http status code with params.

	return runtime.NewGetRuntimeReadinessOK()
}

type getAgentLiveness struct{}

// NewGetAgentRuntimeLivenessHandler handles requests for agent runtime liveness probe.
func NewGetAgentRuntimeLivenessHandler() runtimeops.GetRuntimeLivenessHandler {
	return &getAgentLiveness{}
}

// Handle handles GET requests for /runtime/liveness .
func (g *getAgentLiveness) Handle(params runtimeops.GetRuntimeLivenessParams) middleware.Responder {
	// TODO: return the http status code with params.

	return runtime.NewGetRuntimeLivenessOK()
}
