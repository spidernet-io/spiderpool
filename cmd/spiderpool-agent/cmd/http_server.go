// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strconv"

	"github.com/go-openapi/loads"
	agentOpenAPIServer "github.com/spidernet-io/spiderpool/api/v1/agent/server"
	agentOpenAPIRestapi "github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi"
)

// newAgentOpenAPIServer instantiates a new instance of the agent OpenAPI server on the http.
func newAgentOpenAPIServer() (*agentOpenAPIServer.Server, error) {
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
	api.RuntimeGetRuntimeStartupHandler = &httpGetAgentStartup{}
	api.RuntimeGetRuntimeReadinessHandler = &httpGetAgentReadiness{}
	api.RuntimeGetRuntimeLivenessHandler = &httpGetAgentLiveness{}

	// new agent OpenAPI server with api
	srv := agentOpenAPIServer.NewServer(api)

	// customize server configurations.
	srv.EnabledListeners = []string{"http"}
	port, err := strconv.Atoi(agentContext.HttpPort)
	if nil != err {
		return nil, err
	}
	srv.Port = port

	// configure API and handlers with some default values.
	srv.ConfigureAPI()

	return srv, nil
}
