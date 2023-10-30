// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/event"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	spiderinformers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v2beta1"
	spiderlisters "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/utils"
	stringutil "github.com/spidernet-io/spiderpool/pkg/utils/string"

	"github.com/cilium/cilium/pkg/ipam/option"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	auto    = "auto"
	cluster = "cluster"
	calico  = "calico"
	cilium  = "cilium"
	none    = "none"
)

var SupportedPodCIDRType = []string{auto, cluster, calico, cilium, none}

const (
	calicoIPPoolCRDName = "ippools.crd.projectcalico.org"
	ciliumConfig        = "cilium-config"
	kubeadmConfigMap    = "kubeadm-config"
)

const (
	NotReady = "NotReady"
	Synced   = "Synced"
)

const messageEnqueueCoordiantor = "Enqueue Coordinator"

var InformerLogger *zap.Logger

type CoordinatorController struct {
	Manager         ctrl.Manager
	Client          client.Client
	APIReader       client.Reader
	coordinatorName atomic.Value

	caliCtrlCanncel context.CancelFunc

	CoordinatorLister spiderlisters.SpiderCoordinatorLister
	ConfigmapLister   corelister.ConfigMapLister

	CoordinatorSynced cache.InformerSynced
	ConfigmapSynced   cache.InformerSynced

	Workqueue workqueue.RateLimitingInterface

	LeaderRetryElectGap time.Duration
	ResyncPeriod        time.Duration

	DefaultCniConfDir string
	CiliumConfigMap   string
}

func (cc *CoordinatorController) SetupInformer(
	ctx context.Context,
	spiderClientset clientset.Interface,
	k8sClientset *kubernetes.Clientset,
	leader election.SpiderLeaseElector,
) error {
	if spiderClientset == nil {
		return fmt.Errorf("spiderpoolv2beta1 clientset %w", constant.ErrMissingRequiredParam)
	}
	if k8sClientset == nil {
		return fmt.Errorf("kubernetes clientset %w", constant.ErrMissingRequiredParam)
	}
	if leader == nil {
		return fmt.Errorf("controller leader %w", constant.ErrMissingRequiredParam)
	}

	InformerLogger = logutils.Logger.Named("Coordinator-Informer")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !leader.IsElected() {
				time.Sleep(cc.LeaderRetryElectGap)
				continue
			}

			innerCtx, innerCancel := context.WithCancel(ctx)
			go func() {
				for {
					select {
					case <-innerCtx.Done():
						return
					default:
					}

					if !leader.IsElected() {
						InformerLogger.Warn("Leader lost, stop Coordinator informer")
						innerCancel()
						return
					}
					time.Sleep(cc.LeaderRetryElectGap)
				}
			}()

			InformerLogger.Info("Initialize Coordinator informer")
			k8sInformerFactory := informers.NewSharedInformerFactoryWithOptions(k8sClientset, cc.ResyncPeriod, informers.WithNamespace(metav1.NamespaceSystem))
			spiderInformerFactory := externalversions.NewSharedInformerFactory(spiderClientset, cc.ResyncPeriod)
			err := cc.addEventHandlers(
				spiderInformerFactory.Spiderpool().V2beta1().SpiderCoordinators(),
				k8sInformerFactory.Core().V1().ConfigMaps(),
			)
			if nil != err {
				InformerLogger.Error(err.Error())
				continue
			}

			k8sInformerFactory.Start(innerCtx.Done())
			spiderInformerFactory.Start(innerCtx.Done())
			if err := cc.run(logutils.IntoContext(innerCtx, InformerLogger), 1); err != nil {
				InformerLogger.Sugar().Errorf("failed to run Coordinator informer: %v", err)
				innerCancel()
			}
			InformerLogger.Info("Coordinator informer down")
		}
	}()

	return nil
}

func (cc *CoordinatorController) addEventHandlers(
	coordinatorInformer spiderinformers.SpiderCoordinatorInformer,
	configmapInformer coreinformers.ConfigMapInformer,
) error {
	cc.CoordinatorLister = coordinatorInformer.Lister()
	cc.ConfigmapLister = configmapInformer.Lister()
	cc.CoordinatorSynced = coordinatorInformer.Informer().HasSynced
	cc.ConfigmapSynced = configmapInformer.Informer().HasSynced
	cc.Workqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), constant.KindSpiderCoordinator)

	_, err := coordinatorInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cc.enqueueCoordinatorOnAdd,
		UpdateFunc: cc.enqueueCoordinatorOnUpdate,
		DeleteFunc: nil,
	})
	if err != nil {
		return err
	}

	_, err = configmapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: cc.enqueueCoordinatorOnCiliumConfigChange,
		UpdateFunc: func(old, new interface{}) {
			cc.enqueueCoordinatorOnCiliumConfigChange(new)
		},
		DeleteFunc: nil,
	})
	if err != nil {
		return err
	}

	return nil
}
func (cc *CoordinatorController) enqueueCoordinatorOnAdd(obj interface{}) {
	coord := obj.(*spiderpoolv2beta1.SpiderCoordinator)
	logger := InformerLogger.With(
		zap.String("CoordinatorName", coord.Name),
		zap.String("Operation", "ADD"),
	)

	cc.Workqueue.Add(coord.Name)
	logger.Debug(messageEnqueueCoordiantor)
}

func (cc *CoordinatorController) enqueueCoordinatorOnUpdate(oldObj, newObj interface{}) {
	oldCoord := oldObj.(*spiderpoolv2beta1.SpiderCoordinator)
	newCoord := newObj.(*spiderpoolv2beta1.SpiderCoordinator)
	logger := InformerLogger.With(
		zap.String("CoordinatorName", newCoord.Name),
		zap.String("Operation", "UPDATE"),
	)

	if newCoord.Spec.PodCIDRType != oldCoord.Spec.PodCIDRType && *newCoord.Spec.PodCIDRType != *oldCoord.Spec.PodCIDRType {
		event.EventRecorder.Eventf(
			newCoord,
			corev1.EventTypeNormal,
			"PodCIDRTypeChanged",
			"Pod CIDR type changed from %s to %s", *oldCoord.Spec.PodCIDRType, *newCoord.Spec.PodCIDRType,
		)
	}

	cc.Workqueue.Add(newCoord.Name)
	logger.Debug(messageEnqueueCoordiantor)
}

func (cc *CoordinatorController) enqueueCoordinatorOnCiliumConfigChange(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	if cm.Name != ciliumConfig {
		return
	}

	logger := InformerLogger.With(
		zap.String("ConfigmapName", cm.Name),
		zap.String("Operation", "SYNC"),
	)

	v := cc.coordinatorName.Load()
	cn, ok := v.(string)
	if !ok {
		return
	}

	cc.Workqueue.Add(cn)
	logger.Debug(messageEnqueueCoordiantor)
}

func (cc *CoordinatorController) run(ctx context.Context, workers int) error {
	defer cc.Workqueue.ShutDown()

	if ok := cache.WaitForNamedCacheSync(
		constant.KindSpiderCoordinator,
		ctx.Done(),
		cc.CoordinatorSynced,
		cc.ConfigmapSynced,
	); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, cc.runWorker, time.Second)
	}

	<-ctx.Done()

	return nil
}

func (cc *CoordinatorController) runWorker(ctx context.Context) {
	for cc.processNextWorkItem(ctx) {
	}
}

func (cc *CoordinatorController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := cc.Workqueue.Get()
	if shutdown {
		return false
	}
	defer cc.Workqueue.Done(obj)

	logger := logutils.FromContext(ctx).With(
		zap.String("CoordinatorName", obj.(string)),
		zap.String("Operation", "PROCESS"),
	)

	if err := cc.syncHandler(logutils.IntoContext(ctx, logger), obj.(string)); err != nil {
		logger.Sugar().Warnf("Failed to handle, requeuing: %v", err)
		cc.Workqueue.AddRateLimited(obj)
		return true
	}
	logger.Info("Succeed to SYNC")

	cc.Workqueue.Forget(obj)

	return true
}

func (cc *CoordinatorController) syncHandler(ctx context.Context, coordinatorName string) (err error) {
	logger := logutils.FromContext(ctx)

	v := cc.coordinatorName.Load()
	_, ok := v.(string)
	if !ok {
		cc.coordinatorName.Store(coordinatorName)
	}

	coord, err := cc.CoordinatorLister.Get(coordinatorName)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	coordCopy := coord.DeepCopy()

	coordCopy, err = cc.fetchPodAndServerCIDR(ctx, logger, coordCopy)
	if err != nil {
		logger.Sugar().Errorf("failed to handle spidercoordinator: %v", err)
		return err
	}

	if !reflect.DeepEqual(coordCopy.Status, coord.Status) {
		if err = cc.Client.Status().Patch(ctx, coordCopy, client.MergeFrom(coord)); err != nil {
			logger.Sugar().Errorf("failed to patch spidercoordinator phase: %v", err.Error())
			return err
		}
	}

	return
}

func (cc *CoordinatorController) fetchPodAndServerCIDR(ctx context.Context, logger *zap.Logger, coordCopy *spiderpoolv2beta1.SpiderCoordinator) (*spiderpoolv2beta1.SpiderCoordinator, error) {
	var kubeadmConfig *corev1.ConfigMap
	var err error
	if kubeadmConfig, err = cc.ConfigmapLister.ConfigMaps(metav1.NamespaceSystem).Get(kubeadmConfigMap); err != nil {
		event.EventRecorder.Eventf(
			coordCopy,
			corev1.EventTypeWarning,
			"ClusterNotReady",
			err.Error(),
		)
		setStatus2NoReady(logger, coordCopy)
		return coordCopy, err
	}

	if kubeadmConfig == nil {
		msg := fmt.Sprintf("failed to get configmap: %s/%s", metav1.NamespaceSystem, kubeadmConfigMap)
		event.EventRecorder.Eventf(
			coordCopy,
			corev1.EventTypeWarning,
			"ClusterNotReady",
			msg,
		)

		setStatus2NoReady(logger, coordCopy)
		return coordCopy, errors.New(msg)
	}

	k8sPodCIDR, k8sServiceCIDR := extractK8sCIDR(kubeadmConfig)
	if *coordCopy.Spec.PodCIDRType == auto {
		var podCidrType string
		podCidrType, err = fetchType(cc.DefaultCniConfDir)
		if err != nil {
			if apierrors.IsNotFound(err) {
				event.EventRecorder.Eventf(
					coordCopy,
					corev1.EventTypeWarning,
					"FoundCNIErr",
					"failed to found default CNI",
				)
			}

			setStatus2NoReady(logger, coordCopy)
			return coordCopy, fmt.Errorf("failed to fetch the podCIDRType: %v", err)
		}
		logger.Sugar().Infof("spidercoordinator change podCIDRType from auto to %v", podCidrType)
		coordCopy.Spec.PodCIDRType = &podCidrType
	}

	switch *coordCopy.Spec.PodCIDRType {
	case cluster:
		if cc.caliCtrlCanncel != nil {
			cc.caliCtrlCanncel()
			cc.caliCtrlCanncel = nil
		}
		coordCopy.Status.Phase = Synced
		coordCopy.Status.OverlayPodCIDR = k8sPodCIDR
	case calico:
		if err = cc.fetchCalicoIPPools(ctx, logger, coordCopy); err != nil {
			return coordCopy, err
		}
	case cilium:
		if err = cc.fetchCiliumCIDR(ctx, logger, k8sPodCIDR, coordCopy); err != nil {
			return coordCopy, err
		}
	case none:
		coordCopy.Status.Phase = Synced
		coordCopy.Status.OverlayPodCIDR = []string{}
	}

	coordCopy.Status.ServiceCIDR = k8sServiceCIDR
	return coordCopy, nil
}

func (cc *CoordinatorController) fetchCalicoIPPools(ctx context.Context, logger *zap.Logger, coordCopy *spiderpoolv2beta1.SpiderCoordinator) error {
	var crd apiextensionsv1.CustomResourceDefinition
	err := cc.APIReader.Get(ctx, types.NamespacedName{Name: calicoIPPoolCRDName}, &crd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			event.EventRecorder.Eventf(
				coordCopy,
				corev1.EventTypeWarning,
				"CalicoNotReady",
				"Calico needs to be installed first",
			)
		}

		setStatus2NoReady(logger, coordCopy)
		return err
	}

	if cc.caliCtrlCanncel != nil {
		cc.caliCtrlCanncel()
		cc.caliCtrlCanncel = nil
	}

	var calicoController controller.Controller
	calicoController, err = NewCalicoIPPoolController(cc.Manager, coordCopy.Name)
	if err != nil {
		setStatus2NoReady(logger, coordCopy)
		return err
	}

	ctx, cc.caliCtrlCanncel = context.WithCancel(ctx)
	go func() {
		logger.Info("Starting Calico IPPool controller")
		if err := calicoController.Start(ctx); err != nil {
			logger.Sugar().Errorf("Failed to start Calico IPPool controller: %v", err)
		}
		logger.Info("Shutdown Calico IPPool controller")
		if cc.caliCtrlCanncel != nil {
			cc.caliCtrlCanncel()
			cc.caliCtrlCanncel = nil
		}
	}()

	return nil
}

func (cc *CoordinatorController) fetchCiliumCIDR(ctx context.Context, logger *zap.Logger, k8sPodCIDR []string, coordCopy *spiderpoolv2beta1.SpiderCoordinator) error {
	if cc.caliCtrlCanncel != nil {
		cc.caliCtrlCanncel()
		cc.caliCtrlCanncel = nil
	}

	ns, name := stringutil.ParseNsAndName(cc.CiliumConfigMap)
	if ns == "" && name == "" {
		logger.Sugar().Errorf("invalid ENV %s: %s, unable parse cilium-config configMap", "SPIDERPOOL_CILIUM_CONFIGMAP_NAMESPACE_NAME", cc.CiliumConfigMap)
		setStatus2NoReady(logger, coordCopy)
		return fmt.Errorf("invalid ENV %s: %s, unable parse cilium-config configMap", "SPIDERPOOL_CILIUM_CONFIGMAP_NAMESPACE_NAME", cc.CiliumConfigMap)
	}

	ccm, err := cc.ConfigmapLister.ConfigMaps(ns).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			event.EventRecorder.Eventf(
				coordCopy,
				corev1.EventTypeWarning,
				"CiliumNotReady",
				"Cilium needs to be installed first",
			)
		}

		setStatus2NoReady(logger, coordCopy)
		return err
	}

	ipam := ccm.Data["ipam"]
	switch ipam {
	case option.IPAMClusterPool, option.IPAMClusterPoolV2:
		var podCIDR []string
		v4, ok := ccm.Data["cluster-pool-ipv4-cidr"]
		if ok {
			v4 = strings.TrimSpace(v4)
			_, _, err = net.ParseCIDR(v4)
			if err != nil {
				logger.Sugar().Errorf("unable to initialize cluster-pool-ipv4-cidr, invalid CIDR address: %v", v4)
				setStatus2NoReady(logger, coordCopy)
				return err
			}
			podCIDR = append(podCIDR, v4)
		}
		v6, ok := ccm.Data["cluster-pool-ipv6-cidr"]
		if ok {
			v4 = strings.TrimSpace(v6)
			_, _, err = net.ParseCIDR(v6)
			if err != nil {
				logger.Sugar().Errorf("unable to initialize cluster-pool-ipv6-cidr, invalid CIDR address: %v", v4)
				setStatus2NoReady(logger, coordCopy)
				return err
			}
			podCIDR = append(podCIDR, v6)
		}
		coordCopy.Status.OverlayPodCIDR = podCIDR
		coordCopy.Status.Phase = Synced
	case option.IPAMMultiPool:
		// start controller
		ciliumController, err := NewCiliumIPPoolController(cc.Manager, coordCopy.Name)
		if err != nil {
			setStatus2NoReady(logger, coordCopy)
			return err
		}

		ctx, cc.caliCtrlCanncel = context.WithCancel(ctx)
		go func() {
			logger.Info("Starting Cilium IPPool controller")
			if err := ciliumController.Start(ctx); err != nil {
				logger.Sugar().Errorf("Failed to start Cilium IPPool controller: %v", err)
			}
			logger.Info("Shutdown Cilium IPPool controller")
			if cc.caliCtrlCanncel != nil {
				cc.caliCtrlCanncel()
				cc.caliCtrlCanncel = nil
			}
		}()
	case option.IPAMKubernetes:
		coordCopy.Status.OverlayPodCIDR = k8sPodCIDR
		coordCopy.Status.Phase = Synced
	default:
		logger.Sugar().Infof("unsupported CIlium IPAM mode: %v", ipam)
		setStatus2NoReady(logger, coordCopy)
		return fmt.Errorf("unsupported CIlium IPAM mode: %v", ipam)
	}
	return nil
}

func extractK8sCIDR(kubeadmConfig *corev1.ConfigMap) ([]string, []string) {
	var podCIDR, serviceCIDR []string

	podReg := regexp.MustCompile(`podSubnet: (.*)`)
	serviceReg := regexp.MustCompile(`serviceSubnet: (.*)`)

	var podSubnets, serviceSubnets []string
	for _, data := range kubeadmConfig.Data {
		podSubnets = podReg.FindStringSubmatch(data)
		serviceSubnets = serviceReg.FindStringSubmatch(data)
	}

	if len(podSubnets) != 0 {
		for _, cidr := range strings.Split(podSubnets[1], ",") {
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			podCIDR = append(podCIDR, cidr)
		}
	}

	if len(serviceSubnets) != 0 {
		for _, cidr := range strings.Split(serviceSubnets[1], ",") {
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			serviceCIDR = append(serviceCIDR, cidr)
		}
	}

	return podCIDR, serviceCIDR
}

func fetchType(cniDir string) (string, error) {
	defaultCniName, err := utils.GetDefaultCniName(cniDir)
	if err != nil {
		return "", err
	}

	switch defaultCniName {
	case "calico", "k8s-pod-network":
		return calico, nil
	case cilium:
		return cilium, nil
	default:
		return none, nil
	}
}

func setStatus2NoReady(logger *zap.Logger, copy *spiderpoolv2beta1.SpiderCoordinator) {
	if copy.Status.Phase != NotReady {
		logger.Sugar().Infof("set spidercoordinator phase from %s to NotReady", copy.Status.Phase)
		copy.Status.Phase = NotReady
	}
	copy.Status.OverlayPodCIDR = []string{}
	copy.Status.ServiceCIDR = []string{}
}
