// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/google/gops/agent"
	"github.com/grafana/pyroscope-go"
	"go.uber.org/zap"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/pkg/ipam"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/kubevirtmanager"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/networking/sysctl"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

// DaemonMain runs agentContext handlers.
func DaemonMain() {
	// Set logger level and re-init global logger.
	level := logutils.ConvertLogLevel(agentContext.Cfg.LogLevel)
	if level == nil {
		panic(fmt.Sprintf("unknown log level %s\n", agentContext.Cfg.LogLevel))
	}
	if err := logutils.InitStdoutLogger(*level); err != nil {
		panic(fmt.Sprintf("failed to initialize logger level %s: %v\n", agentContext.Cfg.LogLevel, err))
	}
	logger = logutils.Logger.Named(binNameAgent)

	// Print version info for debug.
	if len(agentContext.Cfg.CommitVersion) > 0 {
		logger.Sugar().Infof("CommitVersion: %v", agentContext.Cfg.CommitVersion)
	}
	if len(agentContext.Cfg.CommitTime) > 0 {
		logger.Sugar().Infof("CommitTime: %v", agentContext.Cfg.CommitTime)
	}
	if len(agentContext.Cfg.AppVersion) > 0 {
		logger.Sugar().Infof("AppVersion: %v", agentContext.Cfg.AppVersion)
	}

	// Set golang max procs.
	currentP := runtime.GOMAXPROCS(-1)
	logger.Sugar().Infof("Default max golang procs: %d", currentP)
	if currentP > int(agentContext.Cfg.GoMaxProcs) {
		p := runtime.GOMAXPROCS(int(agentContext.Cfg.GoMaxProcs))
		logger.Sugar().Infof("Change max golang procs to %d", p)
	}

	// Load spiderpool's global Comfigmap.
	if err := agentContext.LoadConfigmap(); err != nil {
		logger.Sugar().Fatal("Failed to load Configmap spiderpool-conf: %v", err)
	}
	logger.Sugar().Infof("Spiderpool-agent config: %+v", agentContext.Cfg)

	// setup sysctls
	if agentContext.Cfg.TuneSysctlConfig {
		if err := sysctlConfig(agentContext.Cfg.EnableIPv4, agentContext.Cfg.EnableIPv6); err != nil {
			logger.Sugar().Fatal(err)
		}
	} else {
		logger.Sugar().Infof("setSysctlConfig is disabled.")
	}

	// Set up gops.
	if agentContext.Cfg.GopsListenPort != "" {
		address := "127.0.0.1:" + agentContext.Cfg.GopsListenPort
		op := agent.Options{
			ShutdownCleanup: true,
			Addr:            address,
		}
		if err := agent.Listen(op); err != nil {
			logger.Sugar().Fatalf("gops failed to listen on %s: %v", address, err)
		}
		defer agent.Close()
		logger.Sugar().Infof("gops is listening on %s", address)
	}

	// Set up pyroscope.
	if agentContext.Cfg.PyroscopeAddress != "" {
		logger.Sugar().Infof("pyroscope works in push mode with server: %s", agentContext.Cfg.PyroscopeAddress)
		node, e := os.Hostname()
		if e != nil || len(node) == 0 {
			logger.Sugar().Fatalf("Failed to get hostname: %v", e)
		}

		// These 2 lines are only required if you're using mutex or block profiling
		runtime.SetMutexProfileFraction(5)
		runtime.SetBlockProfileRate(5)
		_, e = pyroscope.Start(pyroscope.Config{
			ApplicationName: binNameAgent,
			ServerAddress:   agentContext.Cfg.PyroscopeAddress,
			Logger:          nil,
			Tags:            map[string]string{"node": node},
			ProfileTypes: []pyroscope.ProfileType{
				pyroscope.ProfileCPU,
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileInuseSpace,
				// additional
				pyroscope.ProfileGoroutines,
				pyroscope.ProfileMutexCount,
				pyroscope.ProfileMutexDuration,
				pyroscope.ProfileBlockCount,
				pyroscope.ProfileBlockDuration,
			},
		})
		if e != nil {
			logger.Sugar().Fatalf("Failed to setup pyroscope: %v", e)
		}
	}

	agentContext.InnerCtx, agentContext.InnerCancel = context.WithCancel(context.Background())
	if err := waitAPIServerReady(agentContext.InnerCtx); err != nil {
		logger.Fatal(err.Error())
	}

	logger.Info("Begin to initialize spiderpool-agent metrics HTTP server")
	initAgentMetricsServer(agentContext.InnerCtx)

	logger.Info("Begin to initialize spiderpool-agent runtime manager")
	mgr, err := newCRDManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.CRDManager = mgr

	// init managers...
	initAgentServiceManagers(agentContext.InnerCtx)

	logger.Info("Begin to initialize IPAM")
	ipamConfig := ipam.IPAMConfig{
		EnableIPv4:             agentContext.Cfg.EnableIPv4,
		EnableIPv6:             agentContext.Cfg.EnableIPv6,
		EnableSpiderSubnet:     agentContext.Cfg.EnableSpiderSubnet,
		EnableStatefulSet:      agentContext.Cfg.EnableStatefulSet,
		EnableKubevirtStaticIP: agentContext.Cfg.EnableKubevirtStaticIP,
		OperationRetries:       agentContext.Cfg.WaitSubnetPoolMaxRetries,
		OperationGapDuration:   time.Duration(agentContext.Cfg.WaitSubnetPoolTime) * time.Second,
		AgentNamespace:         agentContext.Cfg.AgentPodNamespace,
	}
	if len(agentContext.Cfg.MultusClusterNetwork) != 0 {
		ipamConfig.MultusClusterNetwork = ptr.To(agentContext.Cfg.MultusClusterNetwork)
	}
	ipam, err := ipam.NewIPAM(
		ipamConfig,
		agentContext.IPPoolManager,
		agentContext.EndpointManager,
		agentContext.NodeManager,
		agentContext.NSManager,
		agentContext.PodManager,
		agentContext.StsManager,
		agentContext.SubnetManager,
		agentContext.KubevirtManager,
	)
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.IPAM = ipam

	go func() {
		logger.Info("Starting IPAM")
		if err := ipam.Start(agentContext.InnerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	go func() {
		logger.Info("Starting spiderpool-agent runtime manager")
		if err := mgr.Start(agentContext.InnerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()
	waitForCacheSync := mgr.GetCache().WaitForCacheSync(agentContext.InnerCtx)
	if !waitForCacheSync {
		logger.Fatal("failed to wait for syncing controller-runtime cache")
	}

	logger.Info("Begin to initialize spiderpool-agent OpenAPI HTTP server")
	srv, err := newAgentOpenAPIHttpServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.HttpServer = srv

	go func() {
		logger.Info("Starting spiderpool-agent OpenAPI HTTP server")
		if err := srv.Serve(); nil != err {
			if err == http.ErrServerClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	logger.Info("Begin to initialize spiderpool-agent OpenAPI UNIX server")
	// clean up unix socket path legacy, it won't return an error if it doesn't exist
	if err := os.RemoveAll(agentContext.Cfg.IpamUnixSocketPath); err != nil {
		logger.Sugar().Fatalf("Failed to clean up socket %s: %v", agentContext.Cfg.IpamUnixSocketPath, err)
	}
	unixServer, err := newAgentOpenAPIUnixServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.UnixServer = unixServer

	go func() {
		logger.Info("Starting spiderpool-agent OpenAPI UNIX server")
		if err := unixServer.Serve(); nil != err {
			if err == net.ErrClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	spiderpoolAgentAPI, err := openapi.NewAgentOpenAPIUnixClient(agentContext.Cfg.IpamUnixSocketPath)
	if nil != err {
		logger.Fatal(err.Error())
	}
	agentContext.unixClient = spiderpoolAgentAPI

	logger.Info("Set spiderpool-agent startup probe ready")
	agentContext.IsStartupProbe.Store(true)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	WatchSignal(sigCh)
}

// WatchSignal notifies the signal to shut down agentContext handlers.
func WatchSignal(sigCh chan os.Signal) {
	for sig := range sigCh {
		logger.Sugar().Warnw("Received shutdown", "signal", sig)

		// Cancel the internal context of spiderpool-agent.
		// This stops things like the runtime manager, GC, etc.
		if agentContext.InnerCancel != nil {
			agentContext.InnerCancel()
		}

		// shut down agent http server
		if nil != agentContext.HttpServer {
			if err := agentContext.HttpServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("Failed to shutdown spiderpool-agent HTTP server: %v", err)
			}
		}

		// shut down agent unix server
		if nil != agentContext.UnixServer {
			if err := agentContext.UnixServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("Failed to shut down spiderpool-agent UNIX server: %v", err)
			}
		}

		// others...

	}
}

func waitAPIServerReady(ctx context.Context) error {
	config := ctrl.GetConfigOrDie()
	config.APIPath = ""
	config.GroupVersion = nil

	// This client does not query any Kubernetes resources, it is only used
	// to detect whether the API Server's readiness probe is ready, so there
	// is no need to add any decoder.
	config.NegotiatedSerializer = apiruntime.NewSimpleNegotiatedSerializer(apiruntime.SerializerInfo{})

	// Request API Server every 2 seconds until API Server is ready or all 15
	// retries have timed out. (total cost: 2 * 15 = 30s)
	config.Timeout = 2 * time.Second

	client, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return err
	}

	for i := 0; i < 15; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := client.Get().
			AbsPath("/readyz").
			Do(ctx).
			Error()
		if err != nil {
			logger.Sugar().Debugf("API Server not ready: %v", err)
			continue
		}

		return nil
	}

	return errors.New("failed to talk to API Server")
}

func initAgentServiceManagers(ctx context.Context) {
	logger.Debug("Begin to initialize Node manager")
	nodeManager, err := nodemanager.NewNodeManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.NodeManager = nodeManager

	logger.Debug("Begin to initialize Namespace manager")
	nsManager, err := namespacemanager.NewNamespaceManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.NSManager = nsManager

	logger.Debug("Begin to initialize Pod manager")
	podManager, err := podmanager.NewPodManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.PodManager = podManager

	logger.Debug("Begin to initialize StatefulSet manager")
	statefulSetManager, err := statefulsetmanager.NewStatefulSetManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.StsManager = statefulSetManager

	logger.Debug("Begin to initialize Kubevirt manager")
	kubevirtManager := kubevirtmanager.NewKubevirtManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
	)
	agentContext.KubevirtManager = kubevirtManager

	logger.Debug("Begin to initialize Endpoint manager")
	endpointManager, err := workloadendpointmanager.NewWorkloadEndpointManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
		agentContext.Cfg.EnableStatefulSet,
		agentContext.Cfg.EnableKubevirtStaticIP,
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.EndpointManager = endpointManager

	logger.Debug("Begin to initialize ReservedIP manager")
	rIPManager, err := reservedipmanager.NewReservedIPManager(
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.ReservedIPManager = rIPManager

	logger.Debug("Begin to initialize IPPool manager")
	ipPoolManager, err := ippoolmanager.NewIPPoolManager(
		ippoolmanager.IPPoolManagerConfig{
			MaxAllocatedIPs:        &agentContext.Cfg.IPPoolMaxAllocatedIPs,
			EnableKubevirtStaticIP: agentContext.Cfg.EnableKubevirtStaticIP,
		},
		agentContext.CRDManager.GetClient(),
		agentContext.CRDManager.GetAPIReader(),
		agentContext.ReservedIPManager,
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	agentContext.IPPoolManager = ipPoolManager

	if agentContext.Cfg.EnableSpiderSubnet {
		logger.Debug("Begin to initialize Subnet manager")
		subnetManager, err := subnetmanager.NewSubnetManager(
			agentContext.CRDManager.GetClient(),
			agentContext.CRDManager.GetAPIReader(),
			agentContext.ReservedIPManager,
		)
		if err != nil {
			logger.Fatal(err.Error())
		}
		agentContext.SubnetManager = subnetManager
	} else {
		logger.Info("Feature SpiderSubnet is disabled")
	}
}

// sysctlConfig set default sysctl configs,Notice: ignore not exist sysctl configs as
// possible.
func sysctlConfig(enableIPv4, enableIPv6 bool) error {
	// setup default sysctl config
	for _, sc := range sysctl.DefaultSysctlConfig {
		if (enableIPv4 && sc.IsIPv4) || (enableIPv6 && sc.IsIPv6) {
			logger.Info("Setup sysctl", zap.String("sysctl", sc.Name), zap.String("value", sc.Value))
			err := sysctl.SetSysctl(sc.Name, sc.Value)
			if err == nil {
				logger.Debug("success to setup sysctl", zap.String("sysctl", sc.Name), zap.String("value", sc.Value))
				continue
			}

			if !errors.Is(err, os.ErrNotExist) {
				logger.Error("failed to setup sysctl", zap.String("sysctl", sc.Name), zap.String("value", sc.Value), zap.Error(err))
				return err
			}
			logger.Warn("skip to setup sysctl", zap.String("sysctl", sc.Name), zap.String("value", sc.Value), zap.Error(err))
		}
	}
	return nil
}
