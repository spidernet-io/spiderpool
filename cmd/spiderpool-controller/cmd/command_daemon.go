// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-openapi/loads"
	"github.com/jessevdk/go-flags"
	"github.com/spf13/cobra"
	controllerServer "github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-controller/server"
	agentRestapi "github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-controller/server/restapi"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "spiderpool controller daemon",
	Long:  "run spiderpool controller daemon",
	Run: func(cmd *cobra.Command, args []string) {
		startControllerServer()
	},
}

func init() {
	ControllerConfig.BindControllerDaemonFlags(daemonCmd.PersistentFlags())
	ControllerConfig.RegisterEnv()

	rootCmd.AddCommand(daemonCmd)
}

// startAgentServer starts Spiderpool Agent server.
func startControllerServer() {
	srv, err := newServer()
	if nil != err {
		logger.Fatal(err.Error())
	}

	sigCh := make(chan os.Signal, 2)
	shutdownDone := make(chan struct{})

	go func() {
		sig := <-sigCh
		logger.Sugar().Warnw("received shutdown", "signal", sig)

		if err := srv.Shutdown(); nil != err {
			logger.Sugar().Errorf("shutting down agent server failed: %s", err)
		}
		close(shutdownDone)
	}()
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	err = srv.Serve()
	if nil != err {
		if err == http.ErrServerClosed {
			<-shutdownDone
			return
		}
		logger.Fatal(err.Error())
	}
}

// newServer instantiates a new instance of the controller API server on the http.
func newServer() (*controllerServer.Server, error) {
	swaggerSpec, err := loads.Embedded(controllerServer.SwaggerJSON, controllerServer.FlatSwaggerJSON)
	if nil != err {
		return nil, err
	}

	api := agentRestapi.NewSpiderpoolControllerAPIAPI(swaggerSpec)
	api.Logger = func(s string, i ...interface{}) {
		logger.Sugar().Infof(s, i)
	}

	// runtime API
	api.RuntimeGetRuntimeReadinessHandler = NewGetControllerRuntimeReadinessHandler()
	api.RuntimeGetRuntimeStartupHandler = NewGetControllerRuntimeStartupHandler()
	api.RuntimeGetRuntimeLivenessHandler = NewGetControllerRuntimeLivenessHandler()

	srv := controllerServer.NewServer(api)
	srv.EnabledListeners = []string{"http"}

	port, err := strconv.Atoi(ControllerConfig.HttpPort)
	if nil != err {
		return nil, err
	}
	srv.Port = port

	// set up swagger server defaults configurations.
	parser := flags.NewParser(srv, flags.Default)
	srv.ConfigureFlags()
	if _, err := parser.Parse(); err != nil {
		code := 1
		if fe, ok := err.(*flags.Error); ok {
			if fe.Type == flags.ErrHelp {
				code = 0
			}
		}
		os.Exit(code)
	}

	srv.ConfigureAPI()

	return srv, nil
}
