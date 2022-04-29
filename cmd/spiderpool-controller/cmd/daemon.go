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

// DaemonMain runs controllerContext handlers.
func DaemonMain() {
	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go WatchSignal(sigCh)

	logger.Info("Begin to initialize spiderpool-controller controller manager.")
	mgr, err := newControllerManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.ControllerManagerCtx, controllerContext.ControllerManagerCancel = context.WithCancel(context.Background())

	// new controller http server
	srv, err := newControllerOpenAPIServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.HttpServer = srv

	go func() {
		logger.Info("Starting spiderpool-agent controller manager.")
		if err := mgr.Start(controllerContext.ControllerManagerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	// serve controller http
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

// WatchSignal notifies the signal to shut down controllerContext handlers.
func WatchSignal(sigCh chan os.Signal) {
	for sig := range sigCh {
		logger.Sugar().Warnw("received shutdown", "signal", sig)

		// TODO: filter some signals

		// TODO
		if controllerContext.ControllerManagerCancel != nil {
			controllerContext.ControllerManagerCancel()
		}

		// shut down http server
		if nil != controllerContext.HttpServer {
			if err := controllerContext.HttpServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("shutting down controller server failed: %s", err)
			}
		}

		// others...

	}
}
