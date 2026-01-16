// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strconv"

	"github.com/go-openapi/loads"
	controllerOpenAPIClient "github.com/spidernet-io/spiderpool/api/v1/controller/client"
	controllerOpenAPIServer "github.com/spidernet-io/spiderpool/api/v1/controller/server"
	controllerOpenAPIRestapi "github.com/spidernet-io/spiderpool/api/v1/controller/server/restapi"
)

// newControllerOpenAPIServer instantiates a new instance of the controller OpenAPI server on the http.
func newControllerOpenAPIServer() (*controllerOpenAPIServer.Server, error) {
	// read yaml spec
	swaggerSpec, err := loads.Embedded(controllerOpenAPIServer.SwaggerJSON, controllerOpenAPIServer.FlatSwaggerJSON)
	if nil != err {
		return nil, err
	}

	// create new service API
	api := controllerOpenAPIRestapi.NewSpiderpoolControllerAPIAPI(swaggerSpec)

	// set spiderpool logger as api logger
	api.Logger = func(s string, i ...interface{}) {
		logger.Sugar().Infof(s, i)
	}

	// runtime API
	api.RuntimeGetRuntimeStartupHandler = httpGetControllerStartup
	api.RuntimeGetRuntimeReadinessHandler = httpGetControllerReadiness
	api.RuntimeGetRuntimeLivenessHandler = httpGetControllerLiveness

	// new controller OpenAPI server with api
	srv := controllerOpenAPIServer.NewServer(api)

	// customize server configurations.
	srv.EnabledListeners = controllerOpenAPIClient.DefaultSchemes
	port, err := strconv.Atoi(controllerContext.Cfg.HTTPPort)
	if nil != err {
		return nil, err
	}
	srv.Port = port

	// configure API and handlers with some default values.
	srv.ConfigureAPI()

	return srv, nil
}
