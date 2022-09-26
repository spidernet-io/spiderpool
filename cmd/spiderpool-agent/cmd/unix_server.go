// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net"
	"net/http"

	"github.com/go-openapi/loads"
	runtime_client "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/jessevdk/go-flags"
	agentOpenAPIClient "github.com/spidernet-io/spiderpool/api/v1/agent/client"
	agentOpenAPIServer "github.com/spidernet-io/spiderpool/api/v1/agent/server"
	agentOpenAPIRestapi "github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi"
)

// NewAgentOpenAPIUnixServer instantiates a new instance of the agent OpenAPI server on the unix.
func NewAgentOpenAPIUnixServer() (*agentOpenAPIServer.Server, error) {
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
	api.DaemonsetPostIpamIPHandler = unixPostAgentIpamIp
	api.DaemonsetDeleteIpamIPHandler = unixDeleteAgentIpamIp
	api.DaemonsetPostIpamIpsHandler = unixPostAgentIpamIps
	api.DaemonsetDeleteIpamIpsHandler = unixDeleteAgentIpamIps

	// new agent OpenAPI server with api
	srv := agentOpenAPIServer.NewServer(api)

	// set spiderpool-agent Unix server with specified unix socket path.
	srv.SocketPath = flags.Filename(agentContext.Cfg.IpamUnixSocketPath)

	// configure API and handlers with some default values.
	srv.ConfigureAPI()

	return srv, nil
}

// NewAgentOpenAPIUnixClient creates a new instance of the agent OpenAPI unix client.
func NewAgentOpenAPIUnixClient(unixSocketPath string) (*agentOpenAPIClient.SpiderpoolAgentAPI, error) {
	if unixSocketPath == "" {
		return nil, fmt.Errorf("unix socket path must be specified")
	}
	transport := &http.Transport{
		DisableCompression: true,
		Dial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", unixSocketPath)
		},
	}
	httpClient := &http.Client{Transport: transport}
	clientTrans := runtime_client.NewWithClient(unixSocketPath, agentOpenAPIClient.DefaultBasePath,
		agentOpenAPIClient.DefaultSchemes, httpClient)
	client := agentOpenAPIClient.New(clientTrans, strfmt.Default)
	return client, nil
}
