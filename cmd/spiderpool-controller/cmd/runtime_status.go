// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/controller/server/restapi/runtime"
)

type getControllerStartup struct{}

// Handle handles GET requests for /runtime/startup .
func (g *getControllerStartup) Handle(params runtime.GetRuntimeStartupParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeStartupOK()
}

type getControllerReadiness struct{}

// Handle handles GET requests for /runtime/readiness .
func (g *getControllerReadiness) Handle(params runtime.GetRuntimeReadinessParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeReadinessOK()
}

type getControllerLiveness struct{}

// Handle handles GET requests for /runtime/liveness .
func (g *getControllerLiveness) Handle(params runtime.GetRuntimeLivenessParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return runtime.NewGetRuntimeLivenessOK()
}
