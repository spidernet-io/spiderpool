// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/gcmanager"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"

	"github.com/google/gops/agent"
	"github.com/pyroscope-io/client/pyroscope"
)

// DaemonMain runs controllerContext handlers.
func DaemonMain() {
	// reinitialize the logger
	v := logutils.ConvertLogLevel(controllerContext.Cfg.LogLevel)
	if v == nil {
		panic(fmt.Sprintf("unknown log level %s \n", controllerContext.Cfg.LogLevel))
	}
	err := logutils.InitStdoutLogger(*v)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger with level %s, reason=%v \n", controllerContext.Cfg.LogLevel, err))
	}
	logger = logutils.Logger.Named(BinNameController)

	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go WatchSignal(sigCh)

	controllerContext.InnerCtx, controllerContext.InnerCancel = context.WithCancel(context.Background())

	if controllerContext.Cfg.GopsListenPort != "" {
		address := "127.0.0.1:" + controllerContext.Cfg.GopsListenPort
		op := agent.Options{
			ShutdownCleanup: true,
			Addr:            address,
		}
		if err := agent.Listen(op); err != nil {
			logger.Sugar().Fatalf("gops failed to listen on port %s, reason=%v", address, err)
		}
		logger.Sugar().Infof("gops is listening on %s ", address)
		defer agent.Close()
	}

	if controllerContext.Cfg.PyroscopeAddress != "" {
		// push mode ,  push to pyroscope server
		logger.Sugar().Infof("pyroscope works in push mode, server %s ", controllerContext.Cfg.PyroscopeAddress)
		node, e := os.Hostname()
		if e != nil || len(node) == 0 {
			logger.Sugar().Fatalf("failed to get hostname, reason=%v", e)
		}
		_, e = pyroscope.Start(pyroscope.Config{
			ApplicationName: BinNameController,
			ServerAddress:   controllerContext.Cfg.PyroscopeAddress,
			Logger:          pyroscope.StandardLogger,
			Tags:            map[string]string{"node": node},
			ProfileTypes: []pyroscope.ProfileType{
				pyroscope.ProfileCPU,
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileInuseSpace,
			},
		})
		if e != nil {
			logger.Sugar().Fatalf("failed to setup pyroscope, reason=%v", e)
		}
	}

	logger.Info("Begin to initialize spiderpool-controller CRD Manager")
	mgr, err := newCRDManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.CRDManager = mgr

	// init managers...
	initControllerServiceManagers(controllerContext.InnerCtx)

	go func() {
		logger.Info("Starting spiderpool-controller CRD Manager")
		if err := mgr.Start(controllerContext.InnerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	logger.Info("Begin to initialize http server")
	// new controller http server
	srv, err := newControllerOpenAPIServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.HttpServer = srv

	// serve controller http
	go func() {
		if err = srv.Serve(); nil != err {
			if err == http.ErrServerClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	// init IP GC manager
	logger.Debug("Begin to initialize IP GC Manager")
	initGCManager(controllerContext.InnerCtx)

	// ...
	time.Sleep(100 * time.Hour)
}

// WatchSignal notifies the signal to shut down controllerContext handlers.
func WatchSignal(sigCh chan os.Signal) {
	for sig := range sigCh {
		logger.Sugar().Warnw("received shutdown", "signal", sig)

		// TODO (Icarus9913):  filter some signals

		// Cancel the internal context of spiderpool-controller.
		// This stops things like the CRD Manager, GC, etc.
		if controllerContext.InnerCancel != nil {
			controllerContext.InnerCancel()
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

func initControllerServiceManagers(ctx context.Context) {
	logger.Debug("Begin to initialize WorkloadEndpoint Manager")
	historySize := controllerContext.Cfg.WorkloadEndpointMaxHistoryRecords

	wepManager, err := workloadendpointmanager.NewWorkloadEndpointManager(controllerContext.CRDManager.GetClient(), controllerContext.CRDManager, historySize)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.WEPManager = wepManager

	logger.Debug("Begin to initialize ReservedIP Manager")
	rIPManager, err := reservedipmanager.NewReservedIPManager(controllerContext.CRDManager.GetClient(), controllerContext.CRDManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.RIPManager = rIPManager

	logger.Debug("Begin to initialize Node Manager")
	nodeManager, err := nodemanager.NewNodeManager(controllerContext.CRDManager.GetClient(), controllerContext.CRDManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.NodeManager = nodeManager

	logger.Debug("Begin to initialize Namespace Manager")
	nsManager, err := namespacemanager.NewNamespaceManager(controllerContext.CRDManager.GetClient(), controllerContext.CRDManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.NSManager = nsManager

	logger.Debug("Begin to initialize Pod Manager")
	retrys := controllerContext.Cfg.UpdateCRMaxRetrys

	podManager, err := podmanager.NewPodManager(controllerContext.CRDManager.GetClient(), controllerContext.CRDManager, retrys)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.PodManager = podManager

	logger.Debug("Begin to initialize IPPool Manager")
	poolSize := controllerContext.Cfg.IPPoolMaxAllocatedIPs

	ipPoolManager, err := ippoolmanager.NewIPPoolManager(controllerContext.CRDManager.GetClient(), controllerContext.RIPManager, controllerContext.NodeManager, controllerContext.NSManager, controllerContext.PodManager, retrys, poolSize)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.IPPoolManager = ipPoolManager
}

func initGCManager(ctx context.Context) {
	gcManager, err := gcmanager.NewGCManager(ctx, controllerContext.CRDManager.GetClient(), gcIPConfig, controllerContext.CRDManager,
		controllerContext.WEPManager, controllerContext.IPPoolManager, controllerContext.PodManager)
	if nil != err {
		logger.Fatal(err.Error())
	}

	controllerContext.GCManager = gcManager

	err = controllerContext.GCManager.Start(ctx)
	if nil != err {
		logger.Fatal(fmt.Sprintf("start gc manager failed, err: '%v'", err))
	}
}
