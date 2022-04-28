// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
)

type unixPostAgentIpamIp struct{}

// Handle handles POST requests for /ipam/ip .
func (g *unixPostAgentIpamIp) Handle(params daemonset.PostIpamIPParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return daemonset.NewPostIpamIPOK()
}

type unixDeleteAgentIpamIp struct{}

// Handle handles DELETE requests for /ipam/ip .
func (g *unixDeleteAgentIpamIp) Handle(params daemonset.DeleteIpamIPParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return daemonset.NewDeleteIpamIPOK()
}

type unixPostAgentIpamIps struct{}

// Handle handles POST requests for /ipam/ips .
func (g *unixPostAgentIpamIps) Handle(params daemonset.PostIpamIpsParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return daemonset.NewPostIpamIpsOK()
}

type unixDeleteAgentIpamIps struct{}

// Handle handles DELETE requests for /ipam/ips .
func (g *unixDeleteAgentIpamIps) Handle(params daemonset.DeleteIpamIpsParams) middleware.Responder {
	// TODO: return the http status code with logic.

	return daemonset.NewDeleteIpamIpsOK()
}
