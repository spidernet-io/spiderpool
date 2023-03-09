// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/google/gops/agent"
	"github.com/pyroscope-io/client/pyroscope"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

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
	// Set logger level and re-init global logger.
	level := logutils.ConvertLogLevel(controllerContext.Cfg.LogLevel)
	if level == nil {
		panic(fmt.Sprintf("unknown log level %s\n", controllerContext.Cfg.LogLevel))
	}
	if err := logutils.InitStdoutLogger(*level); err != nil {
		panic(fmt.Sprintf("failed to initialize logger level %s: %v\n", controllerContext.Cfg.LogLevel, err))
	}
	logger = logutils.Logger.Named(binNameController)

	// Print version info for debug.
	if len(controllerContext.Cfg.CommitVersion) > 0 {
		logger.Sugar().Infof("CommitVersion: %v", controllerContext.Cfg.CommitVersion)
	}
	if len(controllerContext.Cfg.CommitTime) > 0 {
		logger.Sugar().Infof("CommitTime: %v", controllerContext.Cfg.CommitTime)
	}
	if len(controllerContext.Cfg.AppVersion) > 0 {
		logger.Sugar().Infof("AppVersion: %v", controllerContext.Cfg.AppVersion)
	}

	// Set golang max procs.
	currentP := runtime.GOMAXPROCS(-1)
	logger.Sugar().Infof("Default max golang procs: %d", currentP)
	if currentP > int(controllerContext.Cfg.GoMaxProcs) {
		p := runtime.GOMAXPROCS(int(controllerContext.Cfg.GoMaxProcs))
		logger.Sugar().Infof("Change max golang procs to %d", p)
	}

	// Load spiderpool's global Comfigmap.
	if err := controllerContext.LoadConfigmap(); err != nil {
		logger.Sugar().Fatal("Failed to load Configmap spiderpool-conf: %v", err)
	}
	logger.Sugar().Infof("Spiderpool-controller config: %+v", controllerContext.Cfg)

	// Set up gops.
	if controllerContext.Cfg.GopsListenPort != "" {
		address := "127.0.0.1:" + controllerContext.Cfg.GopsListenPort
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
	if controllerContext.Cfg.PyroscopeAddress != "" {
		logger.Sugar().Infof("pyroscope works in push mode with server: %s", controllerContext.Cfg.PyroscopeAddress)
		node, e := os.Hostname()
		if e != nil || len(node) == 0 {
			logger.Sugar().Fatalf("Failed to get hostname: %v", e)
		}
		_, e = pyroscope.Start(pyroscope.Config{
			ApplicationName: binNameController,
			ServerAddress:   controllerContext.Cfg.PyroscopeAddress,
			Logger:          nil,
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
			logger.Sugar().Fatalf("Failed to setup pyroscope: %v", e)
		}
	}

	controllerContext.InnerCtx, controllerContext.InnerCancel = context.WithCancel(context.Background())
	logger.Info("Begin to initialize spiderpool-controller metrics HTTP server")
	initControllerMetricsServer(controllerContext.InnerCtx)

	logger.Info("Begin to initialize spiderpool-controller runtime manager")
	mgr, err := newCRDManager()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.CRDManager = mgr

	logger.Debug("Begin to initialize K8s clientset")
	clientSet, err := initK8sClientSet()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.ClientSet = clientSet

	logger.Debug("Begin to initialize K8s event recorder")
	event.InitEventRecorder(controllerContext.ClientSet, mgr.GetScheme(), constant.Spiderpool)

	// init managers...
	initControllerServiceManagers(controllerContext.InnerCtx)

	go func() {
		logger.Info("Starting spiderpool-controller runtime manager")
		if err := mgr.Start(controllerContext.InnerCtx); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	logger.Info("Begin to initialize OpenAPI HTTP server")
	srv, err := newControllerOpenAPIServer()
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.HttpServer = srv

	go func() {
		if err = srv.Serve(); nil != err {
			if err == http.ErrServerClosed {
				return
			}
			logger.Fatal(err.Error())
		}
	}()

	logger.Info("Begin to initialize IP GC Manager")
	initGCManager(controllerContext.InnerCtx)

	// TODO (Icarus9913): improve k8s StartupProbe
	logger.Info("Set spiderpool-controller Startup probe ready")
	controllerContext.IsStartupProbe.Store(true)

	// The CRD webhook of Spiderpool must be started before informer, so that
	// informer can normally request to some CRs in the cluster without being
	// disturbed by an abnormal webhook.
	checkWebhookReady()

	setupInformers()

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
		// This stops things like the runtime manager, GC, etc.
		if controllerContext.InnerCancel != nil {
			controllerContext.InnerCancel()
		}

		// shut down http server
		if nil != controllerContext.HttpServer {
			if err := controllerContext.HttpServer.Shutdown(); nil != err {
				logger.Sugar().Errorf("Failed to shutdown spiderpool-controller HTTP server: %v", err)
			}
		}

		// others...

	}
}

func initControllerServiceManagers(ctx context.Context) {
	logger.Debug("Begin to initialize spiderpool-controller leader election")
	initSpiderControllerLeaderElect(controllerContext.InnerCtx)

	logger.Debug("Begin to initialize Node manager")
	nodeManager, err := nodemanager.NewNodeManager(
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.NodeManager = nodeManager

	logger.Debug("Begin to initialize Namespace manager")
	nsManager, err := namespacemanager.NewNamespaceManager(
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.NSManager = nsManager

	logger.Debug("Begin to initialize Pod manager")
	podManager, err := podmanager.NewPodManager(
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.PodManager = podManager

	logger.Info("Begin to initialize StatefulSet manager")
	statefulSetManager, err := statefulsetmanager.NewStatefulSetManager(
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.StsManager = statefulSetManager

	logger.Debug("Begin to initialize Endpoint manager")
	endpointManager, err := workloadendpointmanager.NewWorkloadEndpointManager(
		workloadendpointmanager.EndpointManagerConfig{
			MaxConflictRetries:    controllerContext.Cfg.UpdateCRMaxRetries,
			ConflictRetryUnitTime: time.Duration(controllerContext.Cfg.UpdateCRRetryUnitTime) * time.Millisecond,
		},
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.EndpointManager = endpointManager

	logger.Debug("Begin to initialize ReservedIP manager")
	rIPManager, err := reservedipmanager.NewReservedIPManager(
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.RIPManager = rIPManager

	logger.Debug("Begin to set up ReservedIP webhook")
	if err := (&reservedipmanager.ReservedIPWebhook{
		EnableIPv4: controllerContext.Cfg.EnableIPv4,
		EnableIPv6: controllerContext.Cfg.EnableIPv6,
	}).SetupWebhookWithManager(controllerContext.CRDManager); err != nil {
		logger.Fatal(err.Error())
	}

	logger.Debug("Begin to initialize IPPool manager")
	ipPoolManager, err := ippoolmanager.NewIPPoolManager(
		ippoolmanager.IPPoolManagerConfig{
			MaxConflictRetries:    controllerContext.Cfg.UpdateCRMaxRetries,
			ConflictRetryUnitTime: time.Duration(controllerContext.Cfg.UpdateCRRetryUnitTime) * time.Millisecond,
			MaxAllocatedIPs:       &controllerContext.Cfg.IPPoolMaxAllocatedIPs,
		},
		controllerContext.CRDManager.GetClient(),
		controllerContext.CRDManager.GetAPIReader(),
		controllerContext.RIPManager,
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	controllerContext.IPPoolManager = ipPoolManager

	logger.Debug("Begin to set up IPPool webhook")
	if err := (&ippoolmanager.IPPoolWebhook{
		Client:             controllerContext.CRDManager.GetClient(),
		APIReader:          controllerContext.CRDManager.GetAPIReader(),
		EnableIPv4:         controllerContext.Cfg.EnableIPv4,
		EnableIPv6:         controllerContext.Cfg.EnableIPv6,
		EnableSpiderSubnet: controllerContext.Cfg.EnableSpiderSubnet,
	}).SetupWebhookWithManager(controllerContext.CRDManager); err != nil {
		logger.Fatal(err.Error())
	}

	if controllerContext.Cfg.EnableSpiderSubnet {
		logger.Debug("Begin to initialize Subnet manager")
		subnetManager, err := subnetmanager.NewSubnetManager(
			subnetmanager.SubnetManagerConfig{
				MaxConflictRetries:    controllerContext.Cfg.UpdateCRMaxRetries,
				ConflictRetryUnitTime: time.Duration(controllerContext.Cfg.UpdateCRRetryUnitTime) * time.Millisecond,
			},
			controllerContext.CRDManager.GetClient(),
			controllerContext.CRDManager.GetAPIReader(),
			controllerContext.IPPoolManager,
		)
		if err != nil {
			logger.Fatal(err.Error())
		}
		controllerContext.SubnetManager = subnetManager

		logger.Debug("Begin to set up Subnet webhook")
		if err := (&subnetmanager.SubnetWebhook{
			Client:     controllerContext.CRDManager.GetClient(),
			APIReader:  controllerContext.CRDManager.GetAPIReader(),
			EnableIPv4: controllerContext.Cfg.EnableIPv4,
			EnableIPv6: controllerContext.Cfg.EnableIPv6,
		}).SetupWebhookWithManager(controllerContext.CRDManager); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.Info("Feature SpiderSubnet is disabled")
	}
}

func initGCManager(ctx context.Context) {
	// EnableStatefulSet was determined by Configmap.
	gcIPConfig.EnableStatefulSet = controllerContext.Cfg.EnableStatefulSet
	gcManager, err := gcmanager.NewGCManager(
		ctx,
		controllerContext.ClientSet,
		gcIPConfig,
		controllerContext.EndpointManager,
		controllerContext.IPPoolManager,
		controllerContext.PodManager,
		controllerContext.StsManager,
		controllerContext.Leader,
	)
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.GCManager = gcManager

	if err := controllerContext.GCManager.Start(ctx); err != nil {
		logger.Fatal(err.Error())
	}
}

func initSpiderControllerLeaderElect(ctx context.Context) {
	leaseDuration := time.Duration(controllerContext.Cfg.LeaseDuration) * time.Second
	renewDeadline := time.Duration(controllerContext.Cfg.LeaseRenewDeadline) * time.Second
	leaseRetryPeriod := time.Duration(controllerContext.Cfg.LeaseRetryPeriod) * time.Second
	leaderRetryElectGap := time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second

	leaderElector, err := election.NewLeaseElector(
		controllerContext.Cfg.ControllerPodNamespace,
		constant.SpiderControllerElectorLockName,
		controllerContext.Cfg.ControllerPodName,
		&leaseDuration,
		&renewDeadline,
		&leaseRetryPeriod,
		&leaderRetryElectGap,
	)
	if nil != err {
		logger.Fatal(err.Error())
	}

	err = leaderElector.Run(ctx, controllerContext.ClientSet)
	if nil != err {
		logger.Fatal(err.Error())
	}
	controllerContext.Leader = leaderElector
}

// initK8sClientSet will new kubernetes Clientset
func initK8sClientSet() (*kubernetes.Clientset, error) {
	clientSet, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if nil != err {
		return nil, fmt.Errorf("failed to init K8s clientset: %v", err)
	}

	return clientSet, nil
}

// setupInformers will run IPPool,Subnet... informers,
// because these informers count on webhook
func setupInformers() {
	// start SpiderIPPool informer
	crdClient, err := crdclientset.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Info("Begin to set up IPPool informer")
	ipPoolController := ippoolmanager.NewIPPoolController(
		ippoolmanager.IPPoolControllerConfig{
			EnableIPv4:                    controllerContext.Cfg.EnableIPv4,
			EnableIPv6:                    controllerContext.Cfg.EnableIPv6,
			IPPoolControllerWorkers:       controllerContext.Cfg.IPPoolInformerWorkers,
			EnableSpiderSubnet:            controllerContext.Cfg.EnableSpiderSubnet,
			LeaderRetryElectGap:           time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second,
			MaxWorkqueueLength:            controllerContext.Cfg.IPPoolInformerMaxWorkQueueLength,
			WorkQueueRequeueDelayDuration: time.Duration(controllerContext.Cfg.WorkQueueRequeueDelayDuration) * time.Second,
			WorkQueueMaxRetries:           controllerContext.Cfg.WorkQueueMaxRetries,
			ResyncPeriod:                  time.Duration(controllerContext.Cfg.IPPoolInformerResyncPeriod) * time.Second,
		},
		controllerContext.CRDManager.GetClient(),
		controllerContext.RIPManager,
	)
	err = ipPoolController.SetupInformer(controllerContext.InnerCtx, crdClient, controllerContext.Leader)
	if nil != err {
		logger.Fatal(err.Error())
	}

	if controllerContext.Cfg.EnableSpiderSubnet {
		logger.Info("Begin to set up Subnet informer")
		if err := (&subnetmanager.SubnetController{
			Client:                  controllerContext.CRDManager.GetClient(),
			APIReader:               controllerContext.CRDManager.GetAPIReader(),
			LeaderRetryElectGap:     time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second,
			ResyncPeriod:            time.Duration(controllerContext.Cfg.SubnetResyncPeriod) * time.Second,
			SubnetControllerWorkers: controllerContext.Cfg.SubnetInformerWorkers,
			MaxWorkqueueLength:      controllerContext.Cfg.SubnetInformerMaxWorkqueueLength,
		}).SetupInformer(controllerContext.InnerCtx, crdClient, controllerContext.Leader); err != nil {
			logger.Fatal(err.Error())
		}

		logger.Info("Begin to set up auto-created IPPool controller")
		subnetAppController, err := subnetmanager.NewSubnetAppController(
			controllerContext.CRDManager.GetClient(),
			controllerContext.SubnetManager,
			subnetmanager.SubnetAppControllerConfig{
				EnableIPv4:                    controllerContext.Cfg.EnableIPv4,
				EnableIPv6:                    controllerContext.Cfg.EnableIPv6,
				AppControllerWorkers:          controllerContext.Cfg.SubnetAppControllerWorkers,
				MaxWorkqueueLength:            controllerContext.Cfg.SubnetInformerMaxWorkqueueLength,
				WorkQueueRequeueDelayDuration: time.Duration(controllerContext.Cfg.WorkQueueRequeueDelayDuration) * time.Second,
				LeaderRetryElectGap:           time.Duration(controllerContext.Cfg.LeaseRetryGap) * time.Second,
			})
		if nil != err {
			logger.Fatal(err.Error())
		}

		err = subnetAppController.SetupInformer(controllerContext.InnerCtx, controllerContext.ClientSet, controllerContext.Leader)
		if nil != err {
			logger.Fatal(err.Error())
		}
	}
}

func checkWebhookReady() {
	controllerContext.webhookClient = newWebhookHealthCheckClient()

	// TODO(Icarus9913): [Refactor] give a variable rather than hard code 100
	for i := 1; i <= 100; i++ {
		if i == 100 {
			logger.Fatal("out of the max wait duration for webhook ready in process starting phase")
		}

		err := WebhookHealthyCheck(controllerContext.webhookClient, controllerContext.Cfg.WebhookPort)
		if nil != err {
			logger.Error(err.Error())

			time.Sleep(time.Second)
			continue
		}

		break
	}
}
