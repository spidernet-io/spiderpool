// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"context"
	"fmt"
	"net"
	"net/http"

	runtime_client "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	agentOpenAPIClient "github.com/spidernet-io/spiderpool/api/v1/agent/client"
)

// NewAgentOpenAPIUnixClient creates a new instance of the agent OpenAPI unix client.
func NewAgentOpenAPIUnixClient(unixSocketPath string) (*agentOpenAPIClient.SpiderpoolAgentAPI, error) {
	if unixSocketPath == "" {
		return nil, fmt.Errorf("unix socket path must be specified")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", unixSocketPath)
			},
			DisableKeepAlives: true,
		},
	}
	clientTrans := runtime_client.NewWithClient(unixSocketPath, agentOpenAPIClient.DefaultBasePath,
		agentOpenAPIClient.DefaultSchemes, httpClient)
	client := agentOpenAPIClient.New(clientTrans, strfmt.Default)
	return client, nil
}
