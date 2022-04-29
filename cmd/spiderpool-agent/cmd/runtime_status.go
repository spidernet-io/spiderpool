// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/runtime"
)

// Singleton
var (
	httpGetAgentStartup   = &_httpGetAgentStartup{}
	httpGetAgentReadiness = &_httpGetAgentReadiness{}
	httpGetAgentLiveness  = &_httpGetAgentLiveness{}
)

type _httpGetAgentStartup struct{}

// Handle handles GET requests for /runtime/startup .
func (g *_httpGetAgentStartup) Handle(params runtime.GetRuntimeStartupParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeStartupOK()
}

type _httpGetAgentReadiness struct{}

// Handle handles GET requests for /runtime/readiness .
func (g *_httpGetAgentReadiness) Handle(params runtime.GetRuntimeReadinessParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeReadinessOK()
}

type _httpGetAgentLiveness struct{}

// Handle handles GET requests for /runtime/liveness .
func (g *_httpGetAgentLiveness) Handle(params runtime.GetRuntimeLivenessParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeLivenessOK()
}
