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

	"github.com/google/gops/agent"
	"github.com/pyroscope-io/client/pyroscope"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/pkg/config"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/event"
	"github.com/spidernet-io/spiderpool/pkg/gcmanager"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

// DaemonMain runs controllerContext handlers.
func DaemonMain() {
	// reinitialize the logger
	logLevel := logutils.ConvertLogLevel(controllerContext.Cfg.LogLevel)
	if logLevel == nil {
		panic(fmt.Sprintf("unknown log level %s \n", controllerContext.Cfg.LogLevel))
	}
	err := logutils.InitStdoutLogger(*logLevel)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger with level %s, reason=%v \n", controllerContext.Cfg.LogLevel, err))
	}
	logger = logutils.Logger.Named(BinNameController)

	// load Configmap
	err = controllerContext.LoadConfigmap()
	if nil != err {
		logger.Sugar().Fatalf("failed to load configmap, error: %v", err)
	}

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

	logger.Info("Begin to initialize K8s event recorder")
	recorder, err := event.NewEventRecorder(constant.Spiderpool, mgr.GetConfig(), mgr.GetScheme())
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.Recorder = recorder

	logger.Info("Begin to initialize k8s Clientset")
	clientSet, err := initK8sClientSet()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.ClientSet = clientSet

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

	logger.Info("Begin to initialize spiderpool-controller metrics http server")
	initControllerMetricsServer(controllerContext.InnerCtx)

	// init IP GC manager
	logger.Info("Begin to initialize IP GC Manager")
	initGCManager(controllerContext.InnerCtx)

	// TODO (Icarus9913): improve k8s StartupProbe
	logger.Info("Set spiderpool-controller Startup probe ready")
	controllerContext.IsStartupProbe.Store(true)

	// start notifying signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	WatchSignal(sigCh)
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
	updateCRConfig := config.UpdateCRConfig{
		MaxConflictRetries:    controllerContext.Cfg.UpdateCRMaxRetries,
		ConflictRetryUnitTime: time.Duration(controllerContext.Cfg.UpdateCRRetryUnitTime) * time.Millisecond,
	}

	// init spiderpool controller leader election
	logger.Info("Begin to initialize spiderpool controller leader election")
	initSpiderControllerLeaderElect(controllerContext.InnerCtx)

	logger.Info("Begin to initialize WorkloadEndpoint Manager")
	wepManager, err := workloadendpointmanager.NewWorkloadEndpointManager(&workloadendpointmanager.EndpointManagerConfig{
		UpdateCRConfig:    updateCRConfig,
		MaxHistoryRecords: controllerContext.Cfg.WorkloadEndpointMaxHistoryRecords,
	}, controllerContext.CRDManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.WEPManager = wepManager

	logger.Info("Begin to initialize ReservedIP Manager")
	rIPManager, err := reservedipmanager.NewReservedIPManager(&reservedipmanager.ReservedIPManagerConfig{
		EnableIPv4: controllerContext.Cfg.EnableIPv4,
		EnableIPv6: controllerContext.Cfg.EnableIPv6,
	}, controllerContext.CRDManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.RIPManager = rIPManager

	logger.Info("Begin to set up ReservedIP webhook")
	err = controllerContext.RIPManager.SetupWebhook()
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Info("Begin to initialize Node Manager")
	nodeManager, err := nodemanager.NewNodeManager(controllerContext.CRDManager.GetClient())
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.NodeManager = nodeManager

	logger.Info("Begin to initialize Namespace Manager")
	nsManager, err := namespacemanager.NewNamespaceManager(controllerContext.CRDManager.GetClient())
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.NSManager = nsManager

	logger.Info("Begin to initialize Pod Manager")
	podManager, err := podmanager.NewPodManager(&podmanager.PodManagerConfig{
		UpdateCRConfig: updateCRConfig,
	}, controllerContext.CRDManager.GetClient())
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.PodManager = podManager

	logger.Info("Begin to initialize StatefulSet Manager")
	statefulSetManager, err := statefulsetmanager.NewStatefulSetManager(controllerContext.CRDManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.StsManager = statefulSetManager

	logger.Info("Begin to initialize IPPool Manager")
	ipPoolManager, err := ippoolmanager.NewIPPoolManager(&ippoolmanager.IPPoolManagerConfig{
		EnableIPv4:          controllerContext.Cfg.EnableIPv4,
		EnableIPv6:          controllerContext.Cfg.EnableIPv6,
		UpdateCRConfig:      updateCRConfig,
		EnableSpiderSubnet:  controllerContext.Cfg.EnableSpiderSubnet,
		MaxAllocatedIPs:     controllerContext.Cfg.IPPoolMaxAllocatedIPs,
		LeaderRetryElectGap: time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second,
	}, controllerContext.CRDManager, controllerContext.RIPManager)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.IPPoolManager = ipPoolManager

	// start SpiderIPPool informer
	logger.Info("Begin to set up SpiderIPPool informer")
	crdClient, err := crdclientset.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		logger.Fatal(err.Error())
	}
	err = controllerContext.IPPoolManager.SetupInformer(crdClient, controllerContext.Leader)
	if err != nil {
		logger.Fatal(err.Error())
	}

	// set up spiderpool controller IPPool webhook
	logger.Info("Begin to set up SpiderIPPool webhook")
	err = controllerContext.IPPoolManager.SetupWebhook()
	if err != nil {
		logger.Fatal(err.Error())
	}

	if controllerContext.Cfg.EnableSpiderSubnet {
		logger.Info("Begin to initialize Subnet Manager")
		subnetManager, err := subnetmanager.NewSubnetManager(&subnetmanager.SubnetManagerConfig{
			EnableIPv4:          controllerContext.Cfg.EnableIPv4,
			EnableIPv6:          controllerContext.Cfg.EnableIPv6,
			UpdateCRConfig:      updateCRConfig,
			EnableSpiderSubnet:  controllerContext.Cfg.EnableSpiderSubnet,
			LeaderRetryElectGap: time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second,
		}, controllerContext.CRDManager, controllerContext.IPPoolManager)
		if err != nil {
			logger.Fatal(err.Error())
		}
		controllerContext.SubnetManager = subnetManager
		ipPoolManager.InjectSubnetManager(controllerContext.SubnetManager)

		logger.Info("Begin to set up SpiderSubnet informer")
		err = controllerContext.SubnetManager.SetupInformer(controllerContext.InnerCtx, crdClient, controllerContext.Leader)
		if err != nil {
			logger.Fatal(err.Error())
		}

		logger.Info("Begin to set up Subnet webhook")
		err = controllerContext.SubnetManager.SetupWebhook()
		if err != nil {
			logger.Fatal(err.Error())
		}

		go controllerContext.SubnetManager.Run(ctx, controllerContext.ClientSet)
	} else {
		logger.Info("Feature SpiderSubnet is disabled")
	}
}

func initGCManager(ctx context.Context) {
	// EnableStatefulSet was determined by configmap
	gcIPConfig.EnableStatefulSet = controllerContext.Cfg.EnableStatefulSet

	gcManager, err := gcmanager.NewGCManager(ctx, controllerContext.ClientSet, gcIPConfig, controllerContext.WEPManager,
		controllerContext.IPPoolManager, controllerContext.PodManager, controllerContext.StsManager, controllerContext.Leader)
	if nil != err {
		logger.Fatal(err.Error())
	}

	controllerContext.GCManager = gcManager

	err = controllerContext.GCManager.Start(ctx)
	if nil != err {
		logger.Fatal(fmt.Sprintf("start gc manager failed, err: '%v'", err))
	}
}

func initSpiderControllerLeaderElect(ctx context.Context) {
	leaseDuration := time.Duration(controllerContext.Cfg.LeaseDuration) * time.Second
	renewDeadline := time.Duration(controllerContext.Cfg.LeaseRenewDeadline) * time.Second
	leaseRetryPeriod := time.Duration(controllerContext.Cfg.LeaseRetryPeriod) * time.Second
	leaderRetryElectGap := time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second
	leaderElector, err := election.NewLeaseElector(ctx, controllerContext.Cfg.ControllerPodNamespace, constant.SpiderControllerElectorLockName,
		controllerContext.Cfg.ControllerPodName, &leaseDuration, &renewDeadline, &leaseRetryPeriod, &leaderRetryElectGap)
	if nil != err {
		logger.Fatal(err.Error())
	}

	controllerContext.Leader = leaderElector
}

// initK8sClientSet will new kubernetes Clientset
func initK8sClientSet() (*kubernetes.Clientset, error) {
	k8sConfig, err := rest.InClusterConfig()
	if nil != err {
		return nil, fmt.Errorf("failed to get k8s cluster config")
	}
	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if nil != err {
		return nil, fmt.Errorf("failed to new k8s ClientSet")
	}

	return clientSet, nil
}
