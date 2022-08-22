// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strconv"

	"github.com/go-openapi/loads"
	agentOpenAPIClient "github.com/spidernet-io/spiderpool/api/v1/agent/client"
	agentOpenAPIServer "github.com/spidernet-io/spiderpool/api/v1/agent/server"
	agentOpenAPIRestapi "github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi"
)

// newAgentOpenAPIHttpServer instantiates a new instance of the agent OpenAPI server on the http.
func newAgentOpenAPIHttpServer() (*agentOpenAPIServer.Server, error) {
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

	// runtime API
	api.RuntimeGetRuntimeStartupHandler = httpGetAgentStartup
	api.RuntimeGetRuntimeReadinessHandler = httpGetAgentReadiness
	api.RuntimeGetRuntimeLivenessHandler = httpGetAgentLiveness

	// new agent OpenAPI server with api
	srv := agentOpenAPIServer.NewServer(api)

	// spiderpool-agent component owns Unix server and Http server, the Unix server uses for IPAM plugin interaction,
	// and the Http server uses for K8s or CLI command.
	// In spider-agent openapi.yaml, we already set x-schemes with value 'unix', so we need set Http server's listener with value 'http'.
	srv.EnabledListeners = agentOpenAPIClient.DefaultSchemes
	port, err := strconv.Atoi(agentContext.Cfg.HttpPort)
	if nil != err {
		return nil, err
	}
	srv.Port = port

	// configure API and handlers with some default values.
	srv.ConfigureAPI()

	return srv, nil
}
