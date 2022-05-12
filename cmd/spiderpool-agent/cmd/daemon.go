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

	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/pkg/ipam"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

// DaemonMain runs agentContext handlers.
func DaemonMain() {
	// load Configmap
	err := agentContext.LoadConfigmap()
	if nil != err {
		logger.Fatal("Load configmap failed, " + err.Error())
	}
	logger.With(zap.String("IpamUnixSocketPath", agentContext.Cfg.IpamUnixSocketPath),
		zap.Bool("EnabledIPv4", agentContext.Cfg.EnableIPv4),
		zap.Bool("EnabledIPv6", agentContext.Cfg.EnableIPv6),
		zap.Strings("ClusterDefaultIPv4IPPool", agentContext.Cfg.ClusterDefaultIPv4IPPool),
		zap.Strings("ClusterDefaultIPv6IPPool", agentContext.Cfg.ClusterDefaultIPv6IPPool),
		zap.String("NetworkMode", agentContext.Cfg.NetworkMode)).
		Info("Load configmap successfully")

	// TODO (Icarus9913): flush ipam plugin config (deprecated)

	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go WatchSignal(sigCh)

	agentContext.InnerCtx, agentContext.InnerCancel = context.WithCancel(context.Background())

	logger.Info("Begin to initialize spiderpool-agent CRD Manager")
	mgr, err := newCRDManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.CRDManager = mgr

	agentContext.WEManager = workloadendpointmanager.NewWorkloadEndpointManager(mgr.GetClient())
	agentContext.RIPManager = reservedipmanager.NewReservedIPManager(mgr.GetClient())
	agentContext.NodeManager = nodemanager.NewNodeManager(mgr.GetClient())
	agentContext.NSManager = namespacemanager.NewNamespaceManager(mgr.GetClient())
	agentContext.PodManager = podmanager.NewPodManager(mgr.GetClient())
	agentContext.IPPoolManager = ippoolmanager.NewIPPoolManager(mgr.GetClient(), agentContext.RIPManager, agentContext.NodeManager, agentContext.NSManager, agentContext.PodManager)
	agentContext.IPAM = ipam.NewIPAM(ipam.IPAMConfig{
		StatuflsetIPEnable:       false,
		ClusterDefaultIPv4IPPool: agentContext.Cfg.ClusterDefaultIPv4IPPool,
		ClusterDefaultIPv6IPPool: agentContext.Cfg.ClusterDefaultIPv6IPPool,
	}, agentContext.IPPoolManager, agentContext.WEManager, agentContext.NSManager, agentContext.PodManager)

	go func() {
		logger.Info("Starting spiderpool-agent CRD Manager")
		if err := mgr.Start(agentContext.InnerCtx); err != nil {
			mgr.GetClient()
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
	// clean up unix socket path legacy, it won't return an error if it doesn't exist
	err = os.RemoveAll(agentContext.Cfg.IpamUnixSocketPath)
	if nil != err {
		logger.Sugar().Fatalf("Error: clean up socket legacy '%s' failed: %v", agentContext.Cfg.IpamUnixSocketPath, err)
	}
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

		// Cancel the internal context of spiderpool-agent.
		// This stops things like the CRD Manager, GC, etc.
		if agentContext.InnerCancel != nil {
			agentContext.InnerCancel()
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
