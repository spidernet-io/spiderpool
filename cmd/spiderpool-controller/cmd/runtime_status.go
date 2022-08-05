// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/controller/server/restapi/runtime"
)

// Singleton
var (
	httpGetControllerStartup   = &_httpGetControllerStartup{controllerContext}
	httpGetControllerReadiness = &_httpGetControllerReadiness{}
	httpGetControllerLiveness  = &_httpGetControllerLiveness{}
)

type _httpGetControllerStartup struct {
	*ControllerContext
}

// Handle handles GET requests for k8s startup probe.
func (g *_httpGetControllerStartup) Handle(params runtime.GetRuntimeStartupParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.
	if g.IsStartupProbe.Load() {
		return runtime.NewGetRuntimeStartupOK()
	}

	return runtime.NewGetRuntimeStartupInternalServerError()
}

type _httpGetControllerReadiness struct{}

// Handle handles GET requests for k8s readiness probe.
func (g *_httpGetControllerReadiness) Handle(params runtime.GetRuntimeReadinessParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return runtime.NewGetRuntimeReadinessOK()
}

type _httpGetControllerLiveness struct{}

// Handle handles GET requests for k8s liveness probe.
func (g *_httpGetControllerLiveness) Handle(params runtime.GetRuntimeLivenessParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return runtime.NewGetRuntimeLivenessOK()
}
