// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"

	"github.com/go-openapi/runtime/middleware"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var ipamAPILogger = logutils.Logger.Named("IPAM")

// Singleton
var (
	unixPostAgentIpamIp    = &_unixPostAgentIpamIp{}
	unixDeleteAgentIpamIp  = &_unixDeleteAgentIpamIp{}
	unixPostAgentIpamIps   = &_unixPostAgentIpamIps{}
	unixDeleteAgentIpamIps = &_unixDeleteAgentIpamIps{}
)

type _unixPostAgentIpamIp struct{}

// Handle handles POST requests for /ipam/ip .
func (g *_unixPostAgentIpamIp) Handle(params daemonset.PostIpamIPParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.
	resp, err := agentContext.IPAM.Allocate(params.HTTPRequest.Context(), params.IpamAddArgs)
	if err != nil {
		ipamAPILogger.Sugar().Errorf("Failed to allocate: %v", err)

		if errors.Is(err, constant.ErrWrongInput) {
			return daemonset.NewPostIpamIPStatus512()
		}
		if errors.Is(err, constant.ErrNotAllocatablePod) {
			return daemonset.NewPostIpamIPStatus513()
		}
		if errors.Is(err, constant.ErrNoAvailablePool) {
			return daemonset.NewPostIpamIPStatus514()
		}
		if errors.Is(err, constant.ErrIPUsedOut) {
			return daemonset.NewPostIpamIPStatus515()
		}

		return daemonset.NewDeleteIpamIPInternalServerError()
	}

	return daemonset.NewPostIpamIPOK().WithPayload(resp)
}

type _unixDeleteAgentIpamIp struct{}

// Handle handles DELETE requests for /ipam/ip .
func (g *_unixDeleteAgentIpamIp) Handle(params daemonset.DeleteIpamIPParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewDeleteIpamIPOK()
}

type _unixPostAgentIpamIps struct{}

// Handle handles POST requests for /ipam/ips .
func (g *_unixPostAgentIpamIps) Handle(params daemonset.PostIpamIpsParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewPostIpamIpsOK()
}

type _unixDeleteAgentIpamIps struct{}

// Handle handles DELETE requests for /ipam/ips .
func (g *_unixDeleteAgentIpamIps) Handle(params daemonset.DeleteIpamIpsParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewDeleteIpamIpsOK()
}
