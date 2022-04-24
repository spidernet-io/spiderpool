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
	agentServer "github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-agent/server"
	agentRestapi "github.com/spidernet-io/spiderpool/api/v1beta/spiderpool-agent/server/restapi"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "spiderpool agent daemon",
	Long:  "run spiderpool agent daemon",
	Run: func(cmd *cobra.Command, args []string) {
		startAgentServer()
	},
}

func init() {
	AgentConfig.BindAgentDaemonFlags(daemonCmd.PersistentFlags())
	AgentConfig.RegisterEnv()

	rootCmd.AddCommand(daemonCmd)
}

// startAgentServer starts Spiderpool Agent server.
func startAgentServer() {
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

// newServer instantiates a new instance of the agent API server on the http.
func newServer() (*agentServer.Server, error) {
	swaggerSpec, err := loads.Embedded(agentServer.SwaggerJSON, agentServer.FlatSwaggerJSON)
	if nil != err {
		return nil, err
	}

	api := agentRestapi.NewSpiderpoolAgentAPIAPI(swaggerSpec)
	api.Logger = func(s string, i ...interface{}) {
		logger.Sugar().Infof(s, i)
	}

	// runtime API
	api.RuntimeGetRuntimeReadinessHandler = NewGetAgentRuntimeReadinessHandler()
	api.RuntimeGetRuntimeStartupHandler = NewGetAgentRuntimeStartupHandler()
	api.RuntimeGetRuntimeLivenessHandler = NewGetAgentRuntimeLivenessHandler()

	srv := agentServer.NewServer(api)
	srv.EnabledListeners = []string{"http"}

	port, err := strconv.Atoi(AgentConfig.HttpPort)
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
