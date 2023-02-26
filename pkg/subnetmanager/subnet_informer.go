// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
)

const (
	MessageEnqueueSubnet = "Enqueue Subnet"
	MessageWorkqueueFull = "Workqueue is full, dropping the element"
)

var InformerLogger *zap.Logger

type SubnetController struct {
	client.Client
	Scheme *runtime.Scheme

	SubnetsLister listers.SpiderSubnetLister
	IPPoolsLister listers.SpiderIPPoolLister

	SubnetIndexer cache.Indexer
	IPPoolIndexer cache.Indexer

	SubnetsSynced cache.InformerSynced
	IPPoolsSynced cache.InformerSynced

	Workqueue workqueue.RateLimitingInterface

	LeaderRetryElectGap     time.Duration
	ResyncPeriod            time.Duration
	SubnetControllerWorkers int
	MaxWorkqueueLength      int
}

func (sc *SubnetController) SetupInformer(ctx context.Context, client clientset.Interface, leader election.SpiderLeaseElector) error {
	if client == nil {
		return fmt.Errorf("spiderpool clientset must be specified")
	}
	if leader == nil {
		return fmt.Errorf("controller leader must be specified")
	}

	InformerLogger = logutils.Logger.Named("Subnet-Informer")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !leader.IsElected() {
				time.Sleep(sc.LeaderRetryElectGap)
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
						InformerLogger.Warn("Leader lost, stop Subnet informer")
						innerCancel()
						return
					}
					time.Sleep(sc.LeaderRetryElectGap)
				}
			}()

			InformerLogger.Info("Initialize Subnet informer")
			informerFactory := externalversions.NewSharedInformerFactory(client, sc.ResyncPeriod)
			sc.addEventHandlers(
				informerFactory.Spiderpool().V1().SpiderSubnets(),
				informerFactory.Spiderpool().V1().SpiderIPPools(),
			)

			informerFactory.Start(innerCtx.Done())
			if err := sc.run(logutils.IntoContext(innerCtx, InformerLogger), sc.SubnetControllerWorkers); err != nil {
				InformerLogger.Sugar().Errorf("failed to run Subnet informer: %v", err)
				innerCancel()
			}
			InformerLogger.Info("Subnet informer down")
		}
	}()

	return nil
}

func (sc *SubnetController) addEventHandlers(subnetInformer informers.SpiderSubnetInformer, ipPoolInformer informers.SpiderIPPoolInformer) {
	sc.SubnetsLister = subnetInformer.Lister()
	sc.IPPoolsLister = ipPoolInformer.Lister()
	sc.SubnetIndexer = subnetInformer.Informer().GetIndexer()
	sc.IPPoolIndexer = ipPoolInformer.Informer().GetIndexer()
	sc.SubnetsSynced = subnetInformer.Informer().HasSynced
	sc.IPPoolsSynced = ipPoolInformer.Informer().HasSynced
	sc.Workqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), constant.SpiderSubnetKind)

	subnetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sc.enqueueSubnetOnAdd,
		UpdateFunc: sc.enqueueSubnetOnUpdate,
		DeleteFunc: nil,
	})

	ipPoolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: sc.enqueueSubnetOnIPPoolChange,
		UpdateFunc: func(old, new interface{}) {
			oldIPPool := old.(*spiderpoolv1.SpiderIPPool)
			newIPPool := new.(*spiderpoolv1.SpiderIPPool)
			if reflect.DeepEqual(newIPPool.Spec.IPs, oldIPPool.Spec.IPs) &&
				reflect.DeepEqual(newIPPool.Spec.ExcludeIPs, oldIPPool.Spec.ExcludeIPs) {
				return
			}
			sc.enqueueSubnetOnIPPoolChange(new)
		},
		DeleteFunc: sc.enqueueSubnetOnIPPoolChange,
	})
}

func (sc *SubnetController) enqueueSubnetOnAdd(obj interface{}) {
	subnet := obj.(*spiderpoolv1.SpiderSubnet)
	logger := InformerLogger.With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "ADD"),
	)

	if sc.Workqueue.Len() >= sc.MaxWorkqueueLength {
		logger.Sugar().Errorf(MessageWorkqueueFull)
		return
	}

	sc.Workqueue.Add(subnet.Name)
	logger.Debug(MessageEnqueueSubnet)
}

func (sc *SubnetController) enqueueSubnetOnUpdate(oldObj, newObj interface{}) {
	newSubnet := newObj.(*spiderpoolv1.SpiderSubnet)
	logger := InformerLogger.With(
		zap.String("SubnetName", newSubnet.Name),
		zap.String("Operation", "UPDATE"),
	)

	if sc.Workqueue.Len() >= sc.MaxWorkqueueLength {
		logger.Sugar().Errorf(MessageWorkqueueFull)
		return
	}

	sc.Workqueue.Add(newSubnet.Name)
	logger.Debug(MessageEnqueueSubnet)
}

func (sc *SubnetController) enqueueSubnetOnIPPoolChange(obj interface{}) {
	ipPool := obj.(*spiderpoolv1.SpiderIPPool)
	ownerSubnet, ok := ipPool.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
	if !ok {
		return
	}

	logger := InformerLogger.With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("SubnetName", ownerSubnet),
		zap.String("Operation", "SYNC"),
	)

	if sc.Workqueue.Len() >= sc.MaxWorkqueueLength {
		logger.Sugar().Errorf(MessageWorkqueueFull)
		return
	}

	sc.Workqueue.Add(ownerSubnet)
	logger.Debug(MessageEnqueueSubnet)
}

func (sc *SubnetController) run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer sc.Workqueue.ShutDown()

	logger := logutils.FromContext(ctx)
	logger.Info("Starting Subnet informer")

	logger.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForNamedCacheSync(constant.SpiderSubnetKind, ctx.Done(), sc.SubnetsSynced, sc.IPPoolsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, sc.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

func (sc *SubnetController) runWorker(ctx context.Context) {
	for sc.processNextWorkItem(ctx) {
	}
}

func (sc *SubnetController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := sc.Workqueue.Get()
	if shutdown {
		return false
	}
	defer sc.Workqueue.Done(obj)

	logger := logutils.FromContext(ctx).With(
		zap.String("SubnetName", obj.(string)),
		zap.String("Operation", "PROCESS"),
	)

	if err := sc.syncHandler(logutils.IntoContext(ctx, logger), obj.(string)); err != nil {
		logger.Sugar().Warnf("Failed to handle, requeuing: %v", err)
		sc.Workqueue.AddRateLimited(obj)
		return true
	}
	sc.Workqueue.Forget(obj)

	return true
}

func (sc *SubnetController) syncHandler(ctx context.Context, subnetName string) error {
	log := logutils.FromContext(ctx)

	subnet, err := sc.SubnetsLister.Get(subnetName)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	subnetCopy := subnet.DeepCopy()
	if err := sc.syncMetadata(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync metadata of Subnet: %v", err)
	}

	// 补label
	if err := sc.syncControllerSubnet(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync reference for controller Subnet: %v", err)
	}

	// 对齐
	//if err := sc.syncControlledIPPoolIPs(ctx, subnetCopy); err != nil {
	log.Info("duiqi44444444444444444444444444444")
	if err := sc.duiqi(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync the IP ranges of controlled IPPools of Subnet: %v", err)
	}

	if subnet.DeletionTimestamp != nil {
		if err := sc.removeFinalizer(ctx, subnetCopy); err != nil {
			return fmt.Errorf("failed to remove finalizer: %v", err)
		}
	}

	return nil
}

func (sc *SubnetController) syncMetadata(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	cidr, err := spiderpoolip.CIDRToLabelValue(*subnet.Spec.IPVersion, subnet.Spec.Subnet)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR %s as a valid label value: %v", subnet.Spec.Subnet, err)
	}

	sync := false
	if v, ok := subnet.Labels[constant.LabelSubnetCIDR]; !ok || v != cidr {
		if subnet.Labels == nil {
			subnet.Labels = make(map[string]string)
		}
		subnet.Labels[constant.LabelSubnetCIDR] = cidr
		sync = true
	}

	if sync {
		if err := sc.Update(ctx, subnet); err != nil {
			return err
		}
	}

	return nil
}

// 补池subnet-owner的label
func (sc *SubnetController) syncControllerSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	ipPools, err := sc.IPPoolsLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, pool := range ipPools {
		if pool.Spec.Subnet != subnet.Spec.Subnet {
			continue
		}

		orphan := false
		poolCopy := pool.DeepCopy()
		if !metav1.IsControlledBy(poolCopy, subnet) {
			if err := ctrl.SetControllerReference(subnet, poolCopy, sc.Scheme); err != nil {
				return err
			}
			orphan = true
		}

		if v, ok := poolCopy.Labels[constant.LabelIPPoolOwnerSpiderSubnet]; !ok || v != subnet.Name {
			if poolCopy.Labels == nil {
				poolCopy.Labels = make(map[string]string)
			}
			poolCopy.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnet.Name
			orphan = true
		}

		if orphan {
			if err := sc.Update(ctx, poolCopy); err != nil {
				return err
			}
		}
	}

	return nil
}

// TODO: 对齐+刷totalIPCount
func (sc *SubnetController) syncControlledIPPoolIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	selector := labels.Set{constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name}.AsSelector()
	ipPools, err := sc.IPPoolsLister.List(selector)
	if err != nil {
		return err
	}

	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return err
	}

	// Merge pre-allocated IP addresses of each IPPool and calculate their count.
	var tmpCount int
	controlledIPPools := spiderpoolv1.PoolIPPreAllocations{}
	for _, pool := range ipPools {
		poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
		if err != nil {
			return err
		}
		if len(poolTotalIPs) == 0 {
			continue
		}

		validIPs := spiderpoolip.IPsIntersectionSet(subnetTotalIPs, poolTotalIPs, false)
		tmpCount += len(validIPs)

		ranges, err := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, validIPs)
		if err != nil {
			return err
		}
		controlledIPPools[pool.Name] = spiderpoolv1.PoolIPPreAllocation{IPs: ranges}
	}

	sync := false
	if !reflect.DeepEqual(controlledIPPools, subnet.Status.ControlledIPPools) {
		allocatedIPCount := int64(tmpCount)
		subnet.Status.AllocatedIPCount = &allocatedIPCount
		subnet.Status.ControlledIPPools = controlledIPPools
		sync = true
	}

	// Update the count of total IP addresses.
	totalIPCount := int64(len(subnetTotalIPs))
	if !reflect.DeepEqual(&totalIPCount, subnet.Status.TotalIPCount) {
		subnet.Status.TotalIPCount = &totalIPCount
		sync = true
	}

	if sync {
		if err := sc.Status().Update(ctx, subnet); err != nil {
			return err
		}

		// Record the metric of how many IPPools the Subnet has.
		metric.SubnetPoolCounts.Record(int64(len(subnet.Status.ControlledIPPools)), attribute.String(constant.SpiderSubnetKind, subnet.Name))
	}

	return nil
}

func (sc *SubnetController) removeFinalizer(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(subnet, metav1.FinalizerDeleteDependents) {
		if err := sc.Delete(
			ctx,
			subnet,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
		); err != nil {
			return err
		}

		if err := sc.Get(ctx, types.NamespacedName{Name: subnet.Name}, subnet); err != nil {
			return err
		}
	}

	if !controllerutil.ContainsFinalizer(subnet, constant.SpiderFinalizer) {
		return nil
	}

	// Some IP addresses are still occupied by the controlled IPPools, ignore
	// to remove the finalizer.
	if len(subnet.Status.ControlledIPPools) > 0 {
		return nil
	}

	controllerutil.RemoveFinalizer(subnet, constant.SpiderFinalizer)
	if err := sc.Update(ctx, subnet); err != nil {
		return err
	}
	logger.Sugar().Infof("Remove finalizer %s", constant.SpiderFinalizer)

	return nil
}

func (sc *SubnetController) duiqi(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	log := logutils.FromContext(ctx)
	ipPools, err := sc.IPPoolsLister.List(labels.Set{constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name}.AsSelector())
	if err != nil {
		return err
	}

	// 自动池提前分配的
	preAutoPoolAllocated := make(spiderpoolv1.PoolIPPreAllocations, len(subnet.Status.ControlledIPPools))
	for poolName, poolAllocatedIPs := range subnet.Status.ControlledIPPools {
		preAutoPoolAllocated[poolName] = poolAllocatedIPs
	}
	for _, pool := range ipPools {
		delete(preAutoPoolAllocated, pool.Name)
	}
	for poolName := range preAutoPoolAllocated {
		appKind, appNS, appName, isMatching := parseAutoPoolName(poolName)
		if !isMatching {
			log.Sugar().Warnf("Not matching %s", poolName)
			delete(preAutoPoolAllocated, poolName)
		}
		log.Sugar().Infof("appKind:'%s'; appNS:'%s'; appName:'%s'", appKind, appNS, appName)
		var appErr error
		switch appKind {
		case constant.KindDeployment:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &appsv1.Deployment{})
		case constant.KindReplicaSet:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &appsv1.ReplicaSet{})
		case constant.KindStatefulSet:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &appsv1.StatefulSet{})
		case constant.KindJob:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &batchv1.Job{})
		case constant.KindCronJob:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &batchv1.CronJob{})
		case constant.KindDaemonSet:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &appsv1.DaemonSet{})
		case constant.KindPod:
			appErr = sc.Get(ctx, types.NamespacedName{Namespace: appNS, Name: appName}, &corev1.Pod{})
		default:
		}

		if appErr != nil {
			if apierrors.IsNotFound(appErr) {
				log.Sugar().Infof("app goodbye, poolName '%s'", poolName)
				delete(preAutoPoolAllocated, poolName)
			} else {
				return fmt.Errorf("bad=============: %w", err)
			}
		}
	}

	if len(preAutoPoolAllocated) != 0 {
		log.Sugar().Debugf("SpiderSubnet %s has auto-created IPPool pre-allocation %v", subnet.Name, preAutoPoolAllocated)
	}

	// subnet自己spec的所有ip
	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return err
	}
	subnetTotalIPsCount := int64(len(subnetTotalIPs))

	var subnetAllocatedIPsCount int
	newSubnetAllocations := make(spiderpoolv1.PoolIPPreAllocations, len(ipPools))
	for _, pool := range ipPools {
		poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
		if err != nil {
			return err
		}
		if len(poolTotalIPs) == 0 {
			continue
		}

		validIPs := spiderpoolip.IPsIntersectionSet(subnetTotalIPs, poolTotalIPs, false)
		subnetAllocatedIPsCount += len(validIPs)

		ranges, err := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, validIPs)
		if err != nil {
			return err
		}
		newSubnetAllocations[pool.Name] = spiderpoolv1.PoolIPPreAllocation{IPs: ranges}
	}
	for preAllocatedPoolName, preAllocation := range preAutoPoolAllocated {
		tmpIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, preAllocation.IPs)
		if nil != err {
			return err
		}
		subnetAllocatedIPsCount += len(tmpIPs)
		newSubnetAllocations[preAllocatedPoolName] = preAllocation
	}

	sync := false
	if !reflect.DeepEqual(newSubnetAllocations, subnet.Status.ControlledIPPools) {
		log.With(zap.Any("old", subnet.Status.ControlledIPPools),
			zap.Any("new", newSubnetAllocations)).
			Info("subnet status allocation changed")

		subnet.Status.ControlledIPPools = newSubnetAllocations
		sync = true
	}

	allocatedIPCount := int64(subnetAllocatedIPsCount)
	if !reflect.DeepEqual(&allocatedIPCount, subnet.Status.AllocatedIPCount) {
		log.With(zap.Any("old", subnet.Status.AllocatedIPCount),
			zap.Any("new", allocatedIPCount)).
			Info("subnet status allocated IP Count changed")

		subnet.Status.AllocatedIPCount = &allocatedIPCount
		sync = true
	}

	// Update the count of total IP addresses.
	if !reflect.DeepEqual(&subnetTotalIPsCount, subnet.Status.TotalIPCount) {
		log.With(zap.Any("old", subnet.Status.TotalIPCount),
			zap.Any("new", subnetTotalIPsCount)).
			Info("subnet status total IP Count changed")

		subnet.Status.TotalIPCount = &subnetTotalIPsCount
		sync = true
	}

	if sync {
		log.Sugar().Infof("go to change subnet: %v", subnet)
		return sc.Status().Update(ctx, subnet)
	}

	return nil
}

func parseAutoPoolName(str string) (appKind, appNS, appName string, isMatching bool) {
	splitStr := strings.Split(str, "-")
	if len(splitStr) <= 3 {
		return "", "", "", false
	}
	isMatching = true
	appNS = splitStr[1]
	appName = splitStr[2]

	kindStr := splitStr[0]
	switch kindStr {
	case strings.ToLower(constant.KindDeployment):
		appKind = constant.KindDeployment
	case strings.ToLower(constant.KindReplicaSet):
		appKind = constant.KindReplicaSet
	case strings.ToLower(constant.KindStatefulSet):
		appKind = constant.KindStatefulSet
	case strings.ToLower(constant.KindJob):
		appKind = constant.KindJob
	case strings.ToLower(constant.KindCronJob):
		appKind = constant.KindCronJob
	case strings.ToLower(constant.KindDaemonSet):
		appKind = constant.KindDaemonSet
	case strings.ToLower(constant.KindPod):
		appKind = constant.KindPod
	default:
		appKind = constant.KindUnknown
	}

	return
}
