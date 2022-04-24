// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-controller/server/restapi/runtime"
	runtimeops "github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-controller/server/restapi/runtime"
)

type getControllerStartup struct{}

// NewGetControllerRuntimeStartupHandler handles requests for runtime startup probe.
func NewGetControllerRuntimeStartupHandler() runtimeops.GetRuntimeStartupHandler {
	return &getControllerStartup{}
}

// Handle handles GET requests for /runtime/startup .
func (g *getControllerStartup) Handle(params runtimeops.GetRuntimeStartupParams) middleware.Responder {
	// TODO: return the http status code with params.

	return runtime.NewGetRuntimeStartupOK()
}

type getControllerReadiness struct{}

// NewGetControllerRuntimeReadinessHandler handles requests for runtime readiness probe.
func NewGetControllerRuntimeReadinessHandler() runtimeops.GetRuntimeReadinessHandler {
	return &getControllerReadiness{}
}

// Handle handles GET requests for /runtime/readiness .
func (g *getControllerReadiness) Handle(params runtimeops.GetRuntimeReadinessParams) middleware.Responder {
	// TODO: return the http status code with params.

	return runtime.NewGetRuntimeReadinessOK()
}

type getControllerLiveness struct{}

// NewGetControllerRuntimeLivenessHandler handles requests for runtime liveness probe.
func NewGetControllerRuntimeLivenessHandler() runtimeops.GetRuntimeLivenessHandler {
	return &getControllerLiveness{}
}

// Handle handles GET requests for /runtime/liveness .
func (g *getControllerLiveness) Handle(params runtimeops.GetRuntimeLivenessParams) middleware.Responder {
	// TODO: return the http status code with params.

	return runtime.NewGetRuntimeLivenessOK()
}
