// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DaemonMain runs agentContext handlers.
func DaemonMain() {
	// load Configmap
	err := agentContext.LoadConfigmap()
	if nil != err {
		logger.Fatal(err.Error())
	}

	// TODO: flush ipam plugin config (deprecated)

	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go WatchSignal(sigCh)

	logger.Info("Begin to initialize spiderpool-agent Controller Manager")
	mgr, err := newControllerManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.ControllerManagerCtx, agentContext.ControllerManagerCancel = context.WithCancel(context.Background())

	go func() {
		logger.Info("Starting spiderpool-agent Controller Manager")
		if err := mgr.Start(agentContext.ControllerManagerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	// new agent http server
	logger.Info("Begin to initialize spiderpool-agent openapi http server.")
	srv, err := newAgentOpenAPIHttpServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.HttpServer = srv

	// serve agent http
	go func() {
		logger.Info("Starting spiderpool-agent openapi http server.")
		if err = srv.Serve(); nil != err {
			if err == http.ErrServerClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	// new agent unix server
	logger.Info("Begin to initialize spiderpool-agent openapi unix server.")
	unixServer, err := NewAgentOpenAPIUnixServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.UnixServer = unixServer

	// serve agent unix
	go func() {
		logger.Info("Starting spiderpool-agent openapi unix server.")
		if err = unixServer.Serve(); nil != err {
			if err == net.ErrClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	// ...

	time.Sleep(100 * time.Hour)
}

// WatchSignal notifies the signal to shut down agentContext handlers.
func WatchSignal(sigCh chan os.Signal) {
	for sig := range sigCh {
		logger.Sugar().Warnw("received shutdown", "signal", sig)

		// TODO (Icarus9913): filter some signals

		// Cancel the context of Controller Manager.
		// This stops things like the CRD's Informer, Webhook, Controller, etc.
		if agentContext.ControllerManagerCancel != nil {
			agentContext.ControllerManagerCancel()
		}

		// shut down agent http server
		if nil != agentContext.HttpServer {
			if err := agentContext.HttpServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("shutting down agent http server failed: %s", err)
			}
		}

		// shut down agent unix server
		if nil != agentContext.UnixServer {
			if err := agentContext.UnixServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("shutting down agent unix server failed: %s", err)
			}
		}

		// others...

	}
}
