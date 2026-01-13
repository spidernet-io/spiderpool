// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/go-openapi/loads"
	"github.com/jessevdk/go-flags"

	agentOpenAPIServer "github.com/spidernet-io/spiderpool/api/v1/agent/server"
	agentOpenAPIRestapi "github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi"
)

// newAgentOpenAPIUnixServer instantiates a new instance of the agent OpenAPI server on the unix.
func newAgentOpenAPIUnixServer() (*agentOpenAPIServer.Server, error) {
	// read yaml spec
	swaggerSpec, err := loads.Embedded(agentOpenAPIServer.SwaggerJSON, agentOpenAPIServer.FlatSwaggerJSON)
	if nil != err {
		return nil, err
	}

	// create new service API
	api := agentOpenAPIRestapi.NewSpiderpoolAgentAPIAPI(swaggerSpec)

	// set spiderpool logger as api logger
	api.Logger = func(s string, i ...interface{}) {
		logger.Sugar().Infof(s, i)
	}

	// daemonset API
	api.ConnectivityGetIpamHealthyHandler = unixGetAgentHealth
	api.DaemonsetPostIpamIPHandler = unixPostAgentIpamIP
	api.DaemonsetDeleteIpamIPHandler = unixDeleteAgentIpamIP
	api.DaemonsetPostIpamIpsHandler = unixPostAgentIpamIps
	api.DaemonsetDeleteIpamIpsHandler = unixDeleteAgentIpamIps
	api.DaemonsetGetCoordinatorConfigHandler = unixGetCoordinatorConfig

	// new agent OpenAPI server with api
	srv := agentOpenAPIServer.NewServer(api)

	// set spiderpool-agent Unix server with specified unix socket path.
	srv.SocketPath = flags.Filename(agentContext.Cfg.IpamUnixSocketPath)

	// configure API and handlers with some default values.
	srv.ConfigureAPI()

	return srv, nil
}
