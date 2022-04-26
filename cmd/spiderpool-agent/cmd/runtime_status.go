// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/runtime"
	runtimeops "github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/runtime"
)

type httpGetAgentStartup struct{}

// Handle handles GET requests for /runtime/startup .
func (g *httpGetAgentStartup) Handle(params runtimeops.GetRuntimeStartupParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeStartupOK()
}

type httpGetAgentReadiness struct{}

// Handle handles GET requests for /runtime/readiness .
func (g *httpGetAgentReadiness) Handle(params runtimeops.GetRuntimeReadinessParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeReadinessOK()
}

type httpGetAgentLiveness struct{}

// Handle handles GET requests for /runtime/liveness .
func (g *httpGetAgentLiveness) Handle(params runtimeops.GetRuntimeLivenessParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeLivenessOK()
}
