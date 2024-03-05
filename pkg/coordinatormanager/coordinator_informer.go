// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cilium/cilium/pkg/ipam/option"
	v2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	cilium_externalversions "github.com/cilium/cilium/pkg/k8s/client/informers/externalversions"
	ciliumLister "github.com/cilium/cilium/pkg/k8s/client/listers/cilium.io/v2alpha1"
	"github.com/google/go-cmp/cmp"
	calicov1 "github.com/tigera/operator/pkg/apis/crd.projectcalico.org/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networkingv1alpha1 "k8s.io/api/networking/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkingInformer "k8s.io/client-go/informers/networking/v1alpha1"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	networkingLister "k8s.io/client-go/listers/networking/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/event"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	spiderinformers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v2beta1"
	spiderlisters "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/utils"
	stringutil "github.com/spidernet-io/spiderpool/pkg/utils/string"
)

const (
	auto    = "auto"
	cluster = "cluster"
	calico  = "calico"
	cilium  = "cilium"
	flannel = "flannel"
	none    = "none"
)

var SupportedPodCIDRType = []string{auto, cluster, calico, cilium, none}

const (
	calicoIPPoolCRDName = "ippools.crd.projectcalico.org"
	ciliumIPPoolCRDName = "ciliumpodippools.cilium.io"
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
	K8sClient *kubernetes.Clientset
	Manager   ctrl.Manager
	Client    client.Client
	APIReader client.Reader

	CoordinatorLister spiderlisters.SpiderCoordinatorLister
	ConfigmapLister   corelister.ConfigMapLister
	// only not to nil if the k8s serviceCIDR is enabled
	ServiceCIDRLister networkingLister.ServiceCIDRLister
	// only not to nil if the cilium multu-pool is enabled
	CiliumIPPoolLister ciliumLister.CiliumPodIPPoolLister

	CoordinatorSynced cache.InformerSynced
	ConfigmapSynced   cache.InformerSynced
	// only not nil if the k8s serviceCIDR is enabled
	ServiceCIDRSynced cache.InformerSynced
	// only not to nil if the cilium multu-pool is enabled
	CiliumIPPoolsSynced cache.InformerSynced

	Workqueue workqueue.RateLimitingInterface

	LeaderRetryElectGap time.Duration
	ResyncPeriod        time.Duration

	DefaultCniConfDir      string
	CiliumConfigMap        string
	DefaultCoordinatorName string
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

			cc.Workqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), constant.KindSpiderCoordinator)

			if err := cc.StartWatchPodCIDR(innerCtx, InformerLogger); err != nil {
				InformerLogger.Error(err.Error())
				continue
			}

			InformerLogger.Info("Initialize Coordinator informer")
			k8sInformerFactory := informers.NewSharedInformerFactory(k8sClientset, cc.ResyncPeriod)
			spiderInformerFactory := externalversions.NewSharedInformerFactory(spiderClientset, cc.ResyncPeriod)
			err := cc.addEventHandlers(
				spiderInformerFactory.Spiderpool().V2beta1().SpiderCoordinators(),
				k8sInformerFactory.Core().V1().ConfigMaps(),
				k8sInformerFactory.Networking().V1alpha1().ServiceCIDRs(),
			)
			if err != nil {
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

func (cc *CoordinatorController) StartWatchPodCIDR(ctx context.Context, logger *zap.Logger) error {
	cniType, err := fetchType(cc.DefaultCniConfDir)
	if err != nil {
		return err
	}

	switch cniType {
	case calico:
		if err = cc.WatchCalicoIPPools(ctx, logger); err != nil {
			return err
		}
	case cilium:
		if err = cc.WatchCiliumIPPools(ctx, logger); err != nil {
			return err
		}
	}
	return nil
}

func (cc *CoordinatorController) addEventHandlers(
	coordinatorInformer spiderinformers.SpiderCoordinatorInformer,
	configmapInformer coreinformers.ConfigMapInformer,
	serviceCIDRInformer networkingInformer.ServiceCIDRInformer,
) error {
	cc.CoordinatorLister = coordinatorInformer.Lister()
	cc.ConfigmapLister = configmapInformer.Lister()
	cc.CoordinatorSynced = coordinatorInformer.Informer().HasSynced
	cc.ConfigmapSynced = configmapInformer.Informer().HasSynced

	_, err := coordinatorInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cc.enqueueCoordinatorOnAdd,
		UpdateFunc: cc.enqueueCoordinatorOnUpdate,
		DeleteFunc: nil,
	})
	if err != nil {
		return err
	}

	_, err = configmapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cc.enqueueCoordinatorOnCiliumConfigAdd,
		UpdateFunc: cc.enqueueCoordinatorOnCiliumConfigUpdated,
		DeleteFunc: nil,
	})
	if err != nil {
		return err
	}

	InformerLogger.Debug("Checking if the ServiceCIDR is available in your cluster")
	var serviceCIDR networkingv1alpha1.ServiceCIDRList
	err = cc.APIReader.List(context.TODO(), &serviceCIDR)
	if err != nil {
		InformerLogger.Warn("ServiceCIDR feature is unavailable in your cluster, Don't start the serviceCIDR informer")
		return nil
	}

	InformerLogger.Debug("the ServiceCIDR is available in your cluster, Start the serviceCIDR informer")
	if err = cc.addServiceCIDRHandler(serviceCIDRInformer.Informer()); err != nil {
		return err
	}
	cc.ServiceCIDRLister = serviceCIDRInformer.Lister()
	return nil
}

func (cc *CoordinatorController) addServiceCIDRHandler(serviceCIDRInformer cache.SharedIndexInformer) error {
	_, err := serviceCIDRInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			serviceCidr := obj.(*networkingv1alpha1.ServiceCIDR)
			logger := InformerLogger.With(
				zap.String("ServiceCIDRName", serviceCidr.Name),
				zap.String("Operation", "Add"),
			)

			cc.Workqueue.Add(fmt.Sprintf("ServiceCIDR/%v", serviceCidr.Name))
			logger.Debug(messageEnqueueCoordiantor)
		},
		DeleteFunc: func(obj interface{}) {
			serviceCidr := obj.(*networkingv1alpha1.ServiceCIDR)
			logger := InformerLogger.With(
				zap.String("ServiceCIDRName", serviceCidr.Name),
				zap.String("Operation", "Del"),
			)

			cc.Workqueue.Add(fmt.Sprintf("ServiceCIDR/%v", serviceCidr.Name))
			logger.Debug(messageEnqueueCoordiantor)
		},
	})

	if err != nil {
		return err
	}

	cc.ServiceCIDRSynced = serviceCIDRInformer.HasSynced
	return nil
}

func (cc *CoordinatorController) enqueueCoordinatorOnAdd(obj interface{}) {
	coord := obj.(*spiderpoolv2beta1.SpiderCoordinator)
	logger := InformerLogger.With(
		zap.String("CoordinatorName", coord.Name),
		zap.String("Operation", "ADD"),
	)

	cc.Workqueue.Add(fmt.Sprintf("SpiderCoordinator/%v", coord.Name))
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
		logger.Sugar().Infof("PodCIDRtype changed from %s to %s", *oldCoord.Spec.PodCIDRType, *newCoord.Spec.PodCIDRType)
		cc.Workqueue.Add(fmt.Sprintf("SpiderCoordinator/%v", newCoord.Name))
		logger.Debug(messageEnqueueCoordiantor)
		return
	}

	coord, err := cc.CoordinatorLister.Get(cc.DefaultCoordinatorName)
	if err != nil {
		return
	}

	coordCopy := coord.DeepCopy()
	if reflect.DeepEqual(coordCopy.Status, newCoord.Status) {
		logger.Info("status have no changes, ignore add it to workqueue")
		return
	}

	cc.Workqueue.Add(fmt.Sprintf("SpiderCoordinator/%v", newCoord.Name))
	logger.Debug(messageEnqueueCoordiantor)
}

func (cc *CoordinatorController) enqueueCoordinatorOnCiliumConfigAdd(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	if cm.Name != ciliumConfig {
		return
	}

	logger := InformerLogger.With(
		zap.String("ConfigmapName", cm.Name),
		zap.String("Operation", "Add"),
	)

	cc.Workqueue.Add(fmt.Sprintf("ConfigMap/%v", cm.Name))
	logger.Debug(messageEnqueueCoordiantor)
}

func (cc *CoordinatorController) enqueueCoordinatorOnCiliumConfigUpdated(oldObj, newObj interface{}) {
	oldCm := oldObj.(*corev1.ConfigMap)
	newCm := newObj.(*corev1.ConfigMap)
	if newCm.Name != ciliumConfig {
		return
	}

	if cmp.Diff(oldCm.Data, newCm.Data) == "" {
		return
	}

	logger := InformerLogger.With(
		zap.String("ConfigmapName", newCm.Name),
		zap.String("Operation", "SYNC"),
	)

	cc.Workqueue.Add(fmt.Sprintf("ConfigMap/%v", newCm.Name))
	logger.Debug(messageEnqueueCoordiantor)
}

func (cc *CoordinatorController) run(ctx context.Context, workers int) error {
	defer cc.Workqueue.ShutDown()

	additionalCacheSync := []cache.InformerSynced{cc.CoordinatorSynced, cc.ConfigmapSynced}
	if cc.ServiceCIDRSynced != nil {
		additionalCacheSync = append(additionalCacheSync, cc.ServiceCIDRSynced)
	}

	if cc.CiliumIPPoolsSynced != nil {
		additionalCacheSync = append(additionalCacheSync, cc.CiliumIPPoolsSynced)
	}

	if ok := cache.WaitForNamedCacheSync(
		constant.KindSpiderCoordinator,
		ctx.Done(),
		additionalCacheSync...,
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
		zap.String("Event Key", obj.(string)),
		zap.String("Operation", "PROCESS"),
	)

	if err := cc.syncHandler(logutils.IntoContext(ctx, logger)); err != nil {
		logger.Sugar().Warnf("Failed to handle, requeuing: %v", err)
		cc.Workqueue.AddRateLimited(obj)
		return true
	}
	logger.Info("Succeed to SYNC")

	cc.Workqueue.Forget(obj)
	return true
}

func (cc *CoordinatorController) syncHandler(ctx context.Context) (err error) {
	logger := logutils.FromContext(ctx)

	coord, err := cc.CoordinatorLister.Get(cc.DefaultCoordinatorName)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	coordCopy := cc.updatePodAndServerCIDR(ctx, logger, coord)
	if !reflect.DeepEqual(coordCopy.Status, coord.Status) {
		logger.Sugar().Infof("Patching coordinator's status from %v to %v", coord.Status, coordCopy.Status)
		if err = cc.Client.Status().Patch(ctx, coordCopy, client.MergeFrom(coord)); err != nil {
			logger.Sugar().Errorf("failed to patch spidercoordinator phase: %v", err.Error())
			return err
		}
		logger.Sugar().Infof("Success to patch coordinator's status to %v", coordCopy.Status)
	}

	return
}

func (cc *CoordinatorController) updatePodAndServerCIDR(ctx context.Context, logger *zap.Logger, coord *spiderpoolv2beta1.SpiderCoordinator) *spiderpoolv2beta1.SpiderCoordinator {
	var err error
	coordCopy := coord.DeepCopy()
	podCidrType := *coordCopy.Spec.PodCIDRType
	if podCidrType == auto {
		// TODO(@cyclinder): Do we need watch if /etc/cni/net.d has changed?
		podCidrType, err = fetchType(cc.DefaultCniConfDir)
		if err != nil {
			logger.Error("unable to get CNIType in your cluster")
			setStatus2NoReady(logger, err.Error(), coordCopy)
			return coordCopy
		}
	}

	var cm corev1.ConfigMap
	var k8sPodCIDR, k8sServiceCIDR []string
	if err := cc.APIReader.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceSystem, Name: "kubeadm-config"}, &cm); err == nil {
		logger.Sugar().Infof("Trying to fetch the ClusterCIDR from kube-system/kubeadm-config")
		k8sPodCIDR, k8sServiceCIDR = ExtractK8sCIDRFromKubeadmConfigMap(&cm)
		logger.Sugar().Infof("kubeadm-config configMap k8sPodCIDR %v, k8sServiceCIDR %v", k8sPodCIDR, k8sServiceCIDR)
	} else {
		logger.Sugar().Warnf("failed to get kube-system/kubeadm-config: %v, trying to fetch the ClusterCIDR from kube-controller-manager", err)
		var cmPodList corev1.PodList
		err = cc.APIReader.List(ctx, &cmPodList, client.MatchingLabels{"component": "kube-controller-manager"})
		if err != nil {
			logger.Sugar().Errorf("failed to get kube-controller-manager Pod with label \"component: kube-controller-manager\": %v", err)
			event.EventRecorder.Eventf(
				coordCopy,
				corev1.EventTypeWarning,
				"ClusterNotReady",
				"Neither kubeadm-config ConfigMap nor kube-controller-manager Pod can be found",
			)
			setStatus2NoReady(logger, err.Error(), coordCopy)
			return coordCopy
		}

		if len(cmPodList.Items) == 0 {
			errMsg := "No kube-controller-manager pod found, unable to get clusterCIDR"
			logger.Error(errMsg)
			setStatus2NoReady(logger, errMsg, coordCopy)
			return coordCopy
		}

		k8sPodCIDR, k8sServiceCIDR = ExtractK8sCIDRFromKCMPod(&cmPodList.Items[0])
	}

	logger.Sugar().Infof("Detect podCIDRType is: %v, try to update podCIDR", podCidrType)
	switch podCidrType {
	case cluster:
		coordCopy.Status.OverlayPodCIDR = k8sPodCIDR
	case calico:
		if err = cc.updateCalicoPodCIDR(ctx, coordCopy); err != nil {
			coordCopy.Status.Phase = NotReady
			coordCopy.Status.Reason = err.Error()
			coordCopy.Status.OverlayPodCIDR = []string{}
			return coordCopy
		}
	case cilium:
		if err = cc.updateCiliumPodCIDR(k8sPodCIDR, coordCopy); err != nil {
			coordCopy.Status.Phase = NotReady
			coordCopy.Status.Reason = err.Error()
			coordCopy.Status.OverlayPodCIDR = []string{}
			return coordCopy
		}
	case none:
		coordCopy.Status.Phase = Synced
		coordCopy.Status.OverlayPodCIDR = []string{}
	}

	if err = cc.updateServiceCIDR(logger, coordCopy); err != nil {
		logger.Sugar().Infof("unable to list the serviceCIDR resources: %v, update service cidr from cluster service cidr", err)
		coordCopy.Status.ServiceCIDR = k8sServiceCIDR
	}

	coordCopy.Status.Phase = Synced
	coordCopy.Status.Reason = ""
	return coordCopy
}

func (cc *CoordinatorController) updateCalicoPodCIDR(ctx context.Context, coordinator *spiderpoolv2beta1.SpiderCoordinator) error {
	var ipPoolList calicov1.IPPoolList
	if err := cc.Client.List(ctx, &ipPoolList); err != nil {
		InformerLogger.Error("failed to get calico ippools", zap.Error(err))
		return err
	}

	// sort the list to admit podCIDR has changed.
	sort.Slice(ipPoolList.Items, func(i, j int) bool {
		return ipPoolList.Items[i].CreationTimestamp.UnixNano() < ipPoolList.Items[j].CreationTimestamp.UnixNano()
	})

	podCIDR := make([]string, 0, len(ipPoolList.Items))
	for _, p := range ipPoolList.Items {
		if p.DeletionTimestamp == nil && !p.Spec.Disabled {
			podCIDR = append(podCIDR, p.Spec.CIDR)
		}
	}

	InformerLogger.Sugar().Debugf("Calico IPPools CIDR: %v", podCIDR)

	if coordinator.Status.Phase == Synced && reflect.DeepEqual(coordinator.Status.OverlayPodCIDR, podCIDR) {
		return nil
	}

	coordinator.Status.OverlayPodCIDR = podCIDR
	return nil
}

func (cc *CoordinatorController) WatchCalicoIPPools(ctx context.Context, logger *zap.Logger) error {
	var crd apiextensionsv1.CustomResourceDefinition
	err := cc.APIReader.Get(ctx, types.NamespacedName{Name: calicoIPPoolCRDName}, &crd)
	if err != nil {
		logger.Error("unable to get calico CRDs, please make sure calico installed first")
		return err
	}

	var calicoController controller.Controller
	calicoController, err = NewCalicoIPPoolController(cc.Manager, cc.Workqueue)
	if err != nil {
		return err
	}

	go func() {
		logger.Info("Starting Calico IPPool controller")
		if err := calicoController.Start(ctx); err != nil {
			logger.Sugar().Errorf("Failed to start Calico IPPool controller: %v", err)
		}
		logger.Info("Shutdown Calico IPPool controller")
	}()

	return nil
}

func (cc *CoordinatorController) WatchCiliumIPPools(ctx context.Context, logger *zap.Logger) error {
	var crd apiextensionsv1.CustomResourceDefinition
	err := cc.APIReader.Get(ctx, types.NamespacedName{Name: ciliumIPPoolCRDName}, &crd)
	if err != nil {
		InformerLogger.Warn("CRD ciliumpodippools.cilium.io no found, can't watch cilium ippools resource")
		return nil
	}

	InformerLogger.Sugar().Infof("Init Cilium IPPools Informer")
	clientSet := versioned.NewForConfigOrDie(ctrl.GetConfigOrDie())
	ciliumInformer := cilium_externalversions.NewSharedInformerFactory(clientSet, cc.ResyncPeriod)
	_, err = ciliumInformer.Cilium().V2alpha1().CiliumPodIPPools().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ippool := obj.(*v2alpha1.CiliumPodIPPool)
			logger := InformerLogger.With(
				zap.String("Cilium IPPool", ippool.Name),
				zap.String("Operation", "ADD"),
			)

			cc.Workqueue.Add(ippool.Name)
			logger.Debug(messageEnqueueCoordiantor)
		},
		DeleteFunc: func(obj interface{}) {
			ippool := obj.(*v2alpha1.CiliumPodIPPool)
			logger := InformerLogger.With(
				zap.String("Cilium IPPool", ippool.Name),
				zap.String("Operation", "DEL"),
			)

			cc.Workqueue.Add(ippool.Name)
			logger.Debug(messageEnqueueCoordiantor)
		},
	})

	if err != nil {
		return err
	}

	logger.Info("Starting Cilium IPPool Informer")
	cc.CiliumIPPoolLister = ciliumInformer.Cilium().V2alpha1().CiliumPodIPPools().Lister()
	cc.CiliumIPPoolsSynced = ciliumInformer.Cilium().V2alpha1().CiliumPodIPPools().Informer().HasSynced
	ciliumInformer.Start(ctx.Done())
	return nil
}

func (cc *CoordinatorController) updateCiliumPodCIDR(k8sPodCIDR []string, coordinator *spiderpoolv2beta1.SpiderCoordinator) error {
	ns, name := stringutil.ParseNsAndName(cc.CiliumConfigMap)
	if ns == "" && name == "" {
		InformerLogger.Sugar().Errorf("invalid ENV %s: %s, unable parse cilium-config configMap", "SPIDERPOOL_CILIUM_CONFIGMAP_NAMESPACE_NAME", cc.CiliumConfigMap)
		return fmt.Errorf("invalid ENV %s: %s, unable parse cilium-config configMap", "SPIDERPOOL_CILIUM_CONFIGMAP_NAMESPACE_NAME", cc.CiliumConfigMap)
	}

	ccm, err := cc.ConfigmapLister.ConfigMaps(ns).Get(name)
	if err != nil {
		event.EventRecorder.Eventf(
			coordinator,
			corev1.EventTypeWarning,
			"CiliumNotReady",
			"Cilium needs to be installed first",
		)
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
				InformerLogger.Sugar().Errorf("unable to initialize cluster-pool-ipv4-cidr, invalid CIDR address: %v", v4)
				return err
			}
			podCIDR = append(podCIDR, v4)
		}

		v6, ok := ccm.Data["cluster-pool-ipv6-cidr"]
		if ok {
			v4 = strings.TrimSpace(v6)
			_, _, err = net.ParseCIDR(v6)
			if err != nil {
				InformerLogger.Sugar().Errorf("unable to initialize cluster-pool-ipv6-cidr, invalid CIDR address: %v", v4)
				return err
			}
			podCIDR = append(podCIDR, v6)
		}
		coordinator.Status.OverlayPodCIDR = podCIDR
	case option.IPAMMultiPool:
		if err = cc.fetchCiliumIPPools(coordinator); err != nil {
			return err
		}
	case option.IPAMKubernetes:
		coordinator.Status.OverlayPodCIDR = k8sPodCIDR
		coordinator.Status.Phase = Synced
	default:
		InformerLogger.Sugar().Errorf("unsupported CIlium IPAM mode: %v", ipam)
		return fmt.Errorf("unsupported CIlium IPAM mode: %v", ipam)
	}
	return nil
}

func (cc *CoordinatorController) fetchCiliumIPPools(coordinator *spiderpoolv2beta1.SpiderCoordinator) error {
	ipPoolList, err := cc.CiliumIPPoolLister.List(labels.NewSelector())
	if err != nil {
		return err
	}

	// sort the list to admit podCIDR has changed.
	sort.Slice(ipPoolList, func(i, j int) bool {
		return ipPoolList[i].CreationTimestamp.UnixNano() < ipPoolList[j].CreationTimestamp.UnixNano()
	})

	podCIDR := make([]string, 0, len(ipPoolList))
	for _, p := range ipPoolList {
		if p.DeletionTimestamp == nil {
			for _, cidr := range p.Spec.IPv4.CIDRs {
				podCIDR = append(podCIDR, string(cidr))
			}

			for _, cidr := range p.Spec.IPv6.CIDRs {
				podCIDR = append(podCIDR, string(cidr))
			}
		}
	}

	InformerLogger.Sugar().Debugf("Cilium IPPools CIDR: %v", ipPoolList)
	if coordinator.Status.Phase == Synced && reflect.DeepEqual(coordinator.Status.OverlayPodCIDR, podCIDR) {
		return nil
	}

	coordinator.Status.OverlayPodCIDR = podCIDR
	return nil
}

func (cc *CoordinatorController) updateServiceCIDR(logger *zap.Logger, coordCopy *spiderpoolv2beta1.SpiderCoordinator) error {
	// fetch kubernetes ServiceCIDR
	if cc.ServiceCIDRLister == nil {
		// serviceCIDR feature is disable if ServiceCIDRLister is nil
		return fmt.Errorf("the kubernetes serviceCIDR is disabled")
	}

	svcPoolList, err := cc.ServiceCIDRLister.List(labels.NewSelector())
	if err != nil {
		return err
	}

	// sort the list to admit serviceCIDR has changed due to the order.
	sort.Slice(svcPoolList, func(i, j int) bool {
		return svcPoolList[i].CreationTimestamp.UnixNano() < svcPoolList[j].CreationTimestamp.UnixNano()
	})

	serviceCIDR := make([]string, 0, len(svcPoolList))
	for _, p := range svcPoolList {
		if p.DeletionTimestamp == nil {
			serviceCIDR = append(serviceCIDR, p.Spec.CIDRs...)
		}
	}

	if coordCopy.Status.Phase == Synced && reflect.DeepEqual(coordCopy.Status.ServiceCIDR, serviceCIDR) {
		return nil
	}

	logger.Sugar().Debug("Got service cidrs: ", serviceCIDR)
	coordCopy.Status.ServiceCIDR = serviceCIDR
	return nil
}

func ExtractK8sCIDRFromKubeadmConfigMap(cm *corev1.ConfigMap) ([]string, []string) {
	var podCIDR, serviceCIDR []string

	podReg := regexp.MustCompile(`podSubnet: (.*)`)
	serviceReg := regexp.MustCompile(`serviceSubnet: (.*)`)

	var podSubnets, serviceSubnets []string
	for _, data := range cm.Data {
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

func ExtractK8sCIDRFromKCMPod(kcm *corev1.Pod) ([]string, []string) {
	var podCIDR, serviceCIDR []string

	podReg := regexp.MustCompile(`--cluster-cidr=(.*)`)
	serviceReg := regexp.MustCompile(`--service-cluster-ip-range=(.*)`)

	var podSubnets, serviceSubnets []string
	for _, l := range kcm.Spec.Containers[0].Command {
		if len(podSubnets) == 0 {
			podSubnets = podReg.FindStringSubmatch(l)
		}
		if len(serviceSubnets) == 0 {
			serviceSubnets = serviceReg.FindStringSubmatch(l)
		}
		if len(podSubnets) != 0 && len(serviceSubnets) != 0 {
			break
		}
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
	case flannel:
		return cluster, nil
	default:
		return none, nil
	}
}

func setStatus2NoReady(logger *zap.Logger, reason string, copy *spiderpoolv2beta1.SpiderCoordinator) {
	if copy.Status.Phase != NotReady {
		logger.Sugar().Infof("set spidercoordinator phase from %s to NotReady", copy.Status.Phase)
		copy.Status.Phase = NotReady
	}
	copy.Status.Reason = reason
	copy.Status.OverlayPodCIDR = []string{}
	copy.Status.ServiceCIDR = []string{}
}
