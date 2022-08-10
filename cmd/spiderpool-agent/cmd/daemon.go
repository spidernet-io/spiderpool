// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/gops/agent"
	"github.com/pyroscope-io/client/pyroscope"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/pkg/ipam"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

// DaemonMain runs agentContext handlers.
func DaemonMain() {
	// reinitialize the logger
	v := logutils.ConvertLogLevel(agentContext.Cfg.LogLevel)
	if v == nil {
		panic(fmt.Sprintf("unknown log level %s \n", agentContext.Cfg.LogLevel))
	}
	err := logutils.InitStdoutLogger(*v)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger with level %s, reason=%v \n", agentContext.Cfg.LogLevel, err))
	}
	logger = logutils.Logger.Named(BinNameAgent)

	// load Configmap
	err = agentContext.LoadConfigmap()
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

	agentContext.InnerCtx, agentContext.InnerCancel = context.WithCancel(context.Background())

	if agentContext.Cfg.GopsListenPort != "" {
		address := "127.0.0.1:" + agentContext.Cfg.GopsListenPort
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

	if agentContext.Cfg.PyroscopeAddress != "" {
		// push mode ,  push to pyroscope server
		logger.Sugar().Infof("pyroscope works in push mode, server %s ", agentContext.Cfg.PyroscopeAddress)
		node, e := os.Hostname()
		if e != nil || len(node) == 0 {
			logger.Sugar().Fatalf("failed to get hostname, reason=%v", e)
		}
		_, e = pyroscope.Start(pyroscope.Config{
			ApplicationName: BinNameAgent,
			ServerAddress:   agentContext.Cfg.PyroscopeAddress,
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

	logger.Info("Begin to initialize spiderpool-agent CRD Manager")
	mgr, err := newCRDManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.CRDManager = mgr

	// init managers...
	initAgentServiceManagers(agentContext.InnerCtx)

	logger.Info("Begin to initialize IPAM")
	ipam, err := ipam.NewIPAM(&ipam.IPAMConfig{
		StatuflsetIPEnable:       false,
		EnableIPv4:               agentContext.Cfg.EnableIPv4,
		EnableIPv6:               agentContext.Cfg.EnableIPv6,
		ClusterDefaultIPv4IPPool: agentContext.Cfg.ClusterDefaultIPv4IPPool,
		ClusterDefaultIPv6IPPool: agentContext.Cfg.ClusterDefaultIPv6IPPool,
		LimiterMaxQueueSize:      agentContext.Cfg.LimiterMaxQueueSize,
		LimiterMaxWaitTime:       time.Duration(agentContext.Cfg.LimiterMaxWaitTime) * time.Second,
	}, agentContext.IPPoolManager, agentContext.WEManager, agentContext.NSManager, agentContext.PodManager)
	agentContext.IPAM = ipam

	go func() {
		logger.Info("Starting IPAM")
		if err := ipam.Start(agentContext.InnerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	go func() {
		logger.Info("Starting spiderpool-agent CRD Manager")
		if err := mgr.Start(agentContext.InnerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	// new agent http server
	logger.Info("Begin to initialize spiderpool-agent openapi http server")
	srv, err := newAgentOpenAPIHttpServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.HttpServer = srv

	// serve agent http
	go func() {
		logger.Info("Starting spiderpool-agent openapi http server")
		if err = srv.Serve(); nil != err {
			if err == http.ErrServerClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	// new agent unix server
	logger.Info("Begin to initialize spiderpool-agent openapi unix server")
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
		logger.Info("Starting spiderpool-agent openapi unix server")
		if err = unixServer.Serve(); nil != err {
			if err == net.ErrClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	// TODO (Icarus9913): improve k8s StartupProbe
	agentContext.IsStartupProbe.Store(true)

	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	WatchSignal(sigCh)
}

// WatchSignal notifies the signal to shut down agentContext handlers.
func WatchSignal(sigCh chan os.Signal) {
	for sig := range sigCh {
		logger.Sugar().Warnw("Received shutdown", "signal", sig)

		// TODO (Icarus9913): filter some signals

		// Cancel the internal context of spiderpool-agent.
		// This stops things like the CRD Manager, GC, etc.
		if agentContext.InnerCancel != nil {
			agentContext.InnerCancel()
		}

		// shut down agent http server
		if nil != agentContext.HttpServer {
			if err := agentContext.HttpServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("Shutting down agent http server failed: %s", err)
			}
		}

		// shut down agent unix server
		if nil != agentContext.UnixServer {
			if err := agentContext.UnixServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("Shutting down agent unix server failed: %s", err)
			}
		}

		// others...

	}
}

func initAgentServiceManagers(ctx context.Context) {
	logger.Debug("Begin to initialize WorkloadEndpoint Manager")
	retrys := agentContext.Cfg.UpdateCRMaxRetrys
	unitTime := time.Duration(agentContext.Cfg.UpdateCRRetryUnitTime) * time.Millisecond
	historySize := agentContext.Cfg.WorkloadEndpointMaxHistoryRecords
	weManager, err := workloadendpointmanager.NewWorkloadEndpointManager(agentContext.CRDManager.GetClient(), agentContext.CRDManager.GetScheme(), historySize, retrys, unitTime)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.WEManager = weManager

	logger.Debug("Begin to initialize ReservedIP Manager")
	rIPManager, err := reservedipmanager.NewReservedIPManager(agentContext.CRDManager.GetClient())
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.RIPManager = rIPManager

	logger.Debug("Begin to initialize Node Manager")
	nodeManager, err := nodemanager.NewNodeManager(agentContext.CRDManager.GetClient())
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.NodeManager = nodeManager

	logger.Debug("Begin to initialize Namespace Manager")
	nsManager, err := namespacemanager.NewNamespaceManager(agentContext.CRDManager.GetClient())
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.NSManager = nsManager

	logger.Debug("Begin to initialize Pod Manager")
	podManager, err := podmanager.NewPodManager(agentContext.CRDManager.GetClient(), retrys, unitTime)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.PodManager = podManager

	logger.Debug("Begin to initialize IPPool Manager")
	poolSize := agentContext.Cfg.IPPoolMaxAllocatedIPs
	ipPoolManager, err := ippoolmanager.NewIPPoolManager(agentContext.CRDManager, agentContext.RIPManager, agentContext.NodeManager, agentContext.NSManager, agentContext.PodManager, poolSize, retrys, unitTime)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.IPPoolManager = ipPoolManager
}
