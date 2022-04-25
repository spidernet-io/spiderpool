// This file is safe to edit. Once it exists it will not be overwritten

// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"crypto/tls"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	runtimeops "github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/runtime"
)

//go:generate swagger generate server --target ../../agent --name SpiderpoolAgentAPI --spec ../openapi.yaml --api-package restapi --server-package server --principal interface{} --default-scheme unix --exclude-main

func configureFlags(api *restapi.SpiderpoolAgentAPIAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *restapi.SpiderpoolAgentAPIAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.UseSwaggerUI()
	// To continue using redoc as your UI, uncomment the following line
	// api.UseRedoc()

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	if api.DaemonsetDeleteIpamIPHandler == nil {
		api.DaemonsetDeleteIpamIPHandler = daemonset.DeleteIpamIPHandlerFunc(func(params daemonset.DeleteIpamIPParams) middleware.Responder {
			return middleware.NotImplemented("operation daemonset.DeleteIpamIP has not yet been implemented")
		})
	}
	if api.DaemonsetDeleteIpamIpsHandler == nil {
		api.DaemonsetDeleteIpamIpsHandler = daemonset.DeleteIpamIpsHandlerFunc(func(params daemonset.DeleteIpamIpsParams) middleware.Responder {
			return middleware.NotImplemented("operation daemonset.DeleteIpamIps has not yet been implemented")
		})
	}
	if api.ConnectivityGetIpamHealthyHandler == nil {
		api.ConnectivityGetIpamHealthyHandler = connectivity.GetIpamHealthyHandlerFunc(func(params connectivity.GetIpamHealthyParams) middleware.Responder {
			return middleware.NotImplemented("operation connectivity.GetIpamHealthy has not yet been implemented")
		})
	}
	if api.RuntimeGetRuntimeLivenessHandler == nil {
		api.RuntimeGetRuntimeLivenessHandler = runtimeops.GetRuntimeLivenessHandlerFunc(func(params runtimeops.GetRuntimeLivenessParams) middleware.Responder {
			return middleware.NotImplemented("operation runtime.GetRuntimeLiveness has not yet been implemented")
		})
	}
	if api.RuntimeGetRuntimeReadinessHandler == nil {
		api.RuntimeGetRuntimeReadinessHandler = runtimeops.GetRuntimeReadinessHandlerFunc(func(params runtimeops.GetRuntimeReadinessParams) middleware.Responder {
			return middleware.NotImplemented("operation runtime.GetRuntimeReadiness has not yet been implemented")
		})
	}
	if api.RuntimeGetRuntimeStartupHandler == nil {
		api.RuntimeGetRuntimeStartupHandler = runtimeops.GetRuntimeStartupHandlerFunc(func(params runtimeops.GetRuntimeStartupParams) middleware.Responder {
			return middleware.NotImplemented("operation runtime.GetRuntimeStartup has not yet been implemented")
		})
	}
	if api.DaemonsetGetWorkloadendpointHandler == nil {
		api.DaemonsetGetWorkloadendpointHandler = daemonset.GetWorkloadendpointHandlerFunc(func(params daemonset.GetWorkloadendpointParams) middleware.Responder {
			return middleware.NotImplemented("operation daemonset.GetWorkloadendpoint has not yet been implemented")
		})
	}
	if api.DaemonsetPostIpamIPHandler == nil {
		api.DaemonsetPostIpamIPHandler = daemonset.PostIpamIPHandlerFunc(func(params daemonset.PostIpamIPParams) middleware.Responder {
			return middleware.NotImplemented("operation daemonset.PostIpamIP has not yet been implemented")
		})
	}
	if api.DaemonsetPostIpamIpsHandler == nil {
		api.DaemonsetPostIpamIpsHandler = daemonset.PostIpamIpsHandlerFunc(func(params daemonset.PostIpamIpsParams) middleware.Responder {
			return middleware.NotImplemented("operation daemonset.PostIpamIps has not yet been implemented")
		})
	}

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix".
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation.
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics.
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
