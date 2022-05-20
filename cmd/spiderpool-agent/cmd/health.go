// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/connectivity"
)

// Singleton
var unixGetAgentHealth = &_unixGetAgentHealth{}

type _unixGetAgentHealth struct{}

// Handle handles GET requests for /ipam/healthy .
func (g *_unixGetAgentHealth) Handle(params connectivity.GetIpamHealthyParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return connectivity.NewGetIpamHealthyOK()
}
