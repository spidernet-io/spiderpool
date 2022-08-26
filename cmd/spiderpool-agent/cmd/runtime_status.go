// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/runtime/middleware"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/runtime"
)

// Singleton
var (
	httpGetAgentStartup   = &_httpGetAgentStartup{agentContext}
	httpGetAgentReadiness = &_httpGetAgentReadiness{agentContext}
	httpGetAgentLiveness  = &_httpGetAgentLiveness{agentContext}
)

type _httpGetAgentStartup struct {
	*AgentContext
}

// Handle handles GET requests for k8s startup probe.
func (g *_httpGetAgentStartup) Handle(params runtime.GetRuntimeStartupParams) middleware.Responder {
	if !g.IsStartupProbe.Load() {
		return runtime.NewGetRuntimeStartupInternalServerError()
	}

	return runtime.NewGetRuntimeStartupOK()
}

type _httpGetAgentReadiness struct {
	*AgentContext
}

// Handle handles GET requests for k8s readiness probe.
func (g *_httpGetAgentReadiness) Handle(params runtime.GetRuntimeReadinessParams) middleware.Responder {
	unixClient, err := NewAgentOpenAPIUnixClient(g.Cfg.IpamUnixSocketPath)
	if nil != err {
		logger.Error(err.Error())
		return runtime.NewGetRuntimeReadinessInternalServerError()
	}

	_, err = unixClient.Connectivity.GetIpamHealthy(connectivity.NewGetIpamHealthyParams())
	if nil != err {
		logger.Error(err.Error())
		return runtime.NewGetRuntimeReadinessInternalServerError()
	}

	return runtime.NewGetRuntimeReadinessOK()
}

type _httpGetAgentLiveness struct {
	*AgentContext
}

// Handle handles GET requests for k8s liveness probe.
func (g *_httpGetAgentLiveness) Handle(params runtime.GetRuntimeLivenessParams) middleware.Responder {
	unixClient, err := NewAgentOpenAPIUnixClient(g.Cfg.IpamUnixSocketPath)
	if nil != err {
		logger.Error(err.Error())
		return runtime.NewGetRuntimeLivenessInternalServerError()
	}

	_, err = unixClient.Connectivity.GetIpamHealthy(connectivity.NewGetIpamHealthyParams())
	if nil != err {
		logger.Error(err.Error())
		return runtime.NewGetRuntimeLivenessInternalServerError()
	}

	return runtime.NewGetRuntimeLivenessOK()
}
