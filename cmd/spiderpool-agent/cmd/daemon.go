// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DaemonMain runs agentContext handlers.
func DaemonMain() {
	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go WatchSignal(sigCh)

	logger.Info("Begin to initialize spiderpool-agent controller manager.")
	mgr, err := newControllerManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.ControllerManagerCtx, agentContext.ControllerManagerCancel = context.WithCancel(context.Background())

	// new agent http server
	srv, err := newAgentOpenAPIHttpServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.HttpServer = srv

	go func() {
		logger.Info("Starting spiderpool-agent controller manager.")
		if err := mgr.Start(agentContext.ControllerManagerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	// serve agent http
	go func() {
		if err = srv.Serve(); nil != err {
			if err == http.ErrServerClosed {
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

		// TODO: filter some signals

		// TODO
		if agentContext.ControllerManagerCancel != nil {
			agentContext.ControllerManagerCancel()
		}

		// shut down http server
		if nil != agentContext.HttpServer {
			if err := agentContext.HttpServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("shutting down agent server failed: %s", err)
			}
		}

		// others...

	}
}
