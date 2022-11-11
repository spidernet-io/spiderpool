// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/event"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

const (
	MessageEnqueueSubnet = "Enqueue Subnet"
	MessageWorkqueueFull = "Workqueue is full, dropping the element"
)

var informerLogger *zap.Logger

func (sm *subnetManager) SetupInformer(ctx context.Context, client clientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if client == nil {
		return fmt.Errorf("failed to start Subnet informer, client must be specified")
	}
	if controllerLeader == nil {
		return fmt.Errorf("failed to start Subnet informer, controller leader must be specified")
	}

	informerLogger = logutils.Logger.Named("Subnet-Informer")
	sm.leader = controllerLeader

	informerLogger.Info("Initialize Subnet informer")
	informerFactory := externalversions.NewSharedInformerFactory(client, sm.config.ResyncPeriod)
	subnetController := newSubnetController(
		sm,
		informerFactory.Spiderpool().V1().SpiderSubnets(),
		informerFactory.Spiderpool().V1().SpiderIPPools(),
	)

	go func() {
		for {
			if !sm.leader.IsElected() {
				time.Sleep(sm.config.LeaderRetryElectGap)
				continue
			}

			subnetController.innerCtx, subnetController.innerCancel = context.WithCancel(ctx)
			go func() {
				for {
					if !sm.leader.IsElected() {
						informerLogger.Warn("Leader lost, stop Subnet informer")
						subnetController.innerCancel()
						return
					}
					time.Sleep(sm.config.LeaderRetryElectGap)
				}
			}()

			informerFactory.Start(subnetController.innerCtx.Done())
			if err := subnetController.Run(sm.config.Workers, subnetController.innerCtx.Done()); err != nil {
				subnetController.innerCancel()
				informerLogger.Sugar().Errorf("Subnet informer down: %v", err)
			}
		}
	}()

	return nil
}

func (sc *SubnetController) enqueueSubnetOnAdd(obj interface{}) {
	subnet := obj.(*spiderpoolv1.SpiderSubnet)
	logger := informerLogger.With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "ADD"),
	)

	if sc.workqueue.Len() >= sc.subnetManager.config.MaxWorkqueueLength {
		logger.Sugar().Errorf(MessageWorkqueueFull)
		return
	}

	sc.workqueue.Add(subnet.Name)
	logger.Debug(MessageEnqueueSubnet)
}

func (sc *SubnetController) enqueueSubnetOnUpdate(oldObj, newObj interface{}) {
	oldSubnet := oldObj.(*spiderpoolv1.SpiderSubnet)
	newSubnet := newObj.(*spiderpoolv1.SpiderSubnet)
	if reflect.DeepEqual(newSubnet.Spec.IPs, oldSubnet.Spec.IPs) &&
		reflect.DeepEqual(newSubnet.Spec.ExcludeIPs, oldSubnet.Spec.ExcludeIPs) {
		return
	}

	logger := informerLogger.With(
		zap.String("SubnetName", newSubnet.Name),
		zap.String("Operation", "UPDATE"),
	)

	if sc.workqueue.Len() >= sc.subnetManager.config.MaxWorkqueueLength {
		logger.Sugar().Errorf(MessageWorkqueueFull)
		return
	}

	sc.workqueue.Add(newSubnet.Name)
	logger.Debug(MessageEnqueueSubnet)
}

func (sc *SubnetController) enqueueSubnetOnIPPoolChange(obj interface{}) {
	ipPool := obj.(*spiderpoolv1.SpiderIPPool)
	ownerSubnet, ok := ipPool.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
	if !ok {
		return
	}

	logger := informerLogger.With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("SubnetName", ownerSubnet),
		zap.String("Operation", "SYNC"),
	)

	if sc.workqueue.Len() >= sc.subnetManager.config.MaxWorkqueueLength {
		logger.Sugar().Errorf(MessageWorkqueueFull)
		return
	}

	sc.workqueue.Add(ownerSubnet)
	logger.Debug(MessageEnqueueSubnet)
}

type SubnetController struct {
	innerCtx      context.Context
	innerCancel   context.CancelFunc
	subnetManager *subnetManager

	subnetsLister listers.SpiderSubnetLister
	subnetsSynced cache.InformerSynced
	ipPoolsLister listers.SpiderIPPoolLister
	ipPoolsSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
}

func newSubnetController(subnetManager *subnetManager, subnetInformer informers.SpiderSubnetInformer, ipPoolInformer informers.SpiderIPPoolInformer) *SubnetController {
	controller := &SubnetController{
		subnetManager: subnetManager,
		subnetsLister: subnetInformer.Lister(),
		subnetsSynced: subnetInformer.Informer().HasSynced,
		ipPoolsLister: ipPoolInformer.Lister(),
		ipPoolsSynced: ipPoolInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SpiderSubnets"),
	}

	informerLogger.Info("Setting up event handlers")
	subnetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueueSubnetOnAdd,
		UpdateFunc: controller.enqueueSubnetOnUpdate,
		DeleteFunc: nil,
	})

	ipPoolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSubnetOnIPPoolChange,
		UpdateFunc: func(old, new interface{}) {
			oldIPPool := old.(*spiderpoolv1.SpiderIPPool)
			newIPPool := new.(*spiderpoolv1.SpiderIPPool)
			if reflect.DeepEqual(newIPPool.Spec.IPs, oldIPPool.Spec.IPs) &&
				reflect.DeepEqual(newIPPool.Spec.ExcludeIPs, oldIPPool.Spec.ExcludeIPs) {
				return
			}
			controller.enqueueSubnetOnIPPoolChange(new)
		},
		DeleteFunc: controller.enqueueSubnetOnIPPoolChange,
	})

	return controller
}

func (sc *SubnetController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer sc.workqueue.ShutDown()

	informerLogger.Info("Starting Subnet informer")

	informerLogger.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, sc.subnetsSynced, sc.ipPoolsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	informerLogger.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(sc.runWorker, time.Second, stopCh)
	}

	informerLogger.Info("Started workers")
	<-stopCh
	informerLogger.Info("Shutting down workers")

	return nil
}

func (sc *SubnetController) runWorker() {
	for sc.processNextWorkItem() {
	}
}

func (sc *SubnetController) processNextWorkItem() bool {
	obj, shutdown := sc.workqueue.Get()

	if shutdown {
		return false
	}

	var logger *zap.Logger
	err := func(obj interface{}) error {
		defer sc.workqueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			sc.workqueue.Forget(obj)
			return nil
		}

		logger = informerLogger.With(
			zap.String("SubnetName", key),
			zap.String("Operation", "PROCESS"),
		)

		if err := sc.syncHandler(logutils.IntoContext(sc.innerCtx, logger), key); err != nil {
			sc.workqueue.AddRateLimited(key)
			return fmt.Errorf("failed to handle, requeuing: %v", err)
		}
		sc.workqueue.Forget(obj)

		return nil
	}(obj)

	if err != nil {
		logger.Error(err.Error())
		return true
	}

	return true
}

func (sc *SubnetController) syncHandler(ctx context.Context, subnetName string) error {
	subnet, err := sc.subnetsLister.Get(subnetName)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	if err := sc.syncControllerSubnet(ctx, subnet); err != nil {
		return fmt.Errorf("failed to sync reference for controller Subnet: %v", err)
	}

	subnetCopy := subnet.DeepCopy()
	if err := sc.syncControlledIPPoolIPs(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync the IP ranges of controlled IPPools of Subnet: %v", err)
	}

	if subnet.DeletionTimestamp != nil {
		if err := sc.removeFinalizer(ctx, subnetCopy); err != nil {
			return fmt.Errorf("failed to remove finalizer: %v", err)
		}
		return nil
	}

	if err := sc.notifySubnetIPPool(ctx, subnet); err != nil {
		return fmt.Errorf("failed to notify the IPPools that did not reach the expected state: %v", err)
	}
	event.EventRecorder.Event(subnet, corev1.EventTypeNormal, constant.EventReasonResyncSubnet, "Resynced successfully")

	return nil
}

func (sc *SubnetController) syncControllerSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	ipPools, err := sc.ipPoolsLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, pool := range ipPools {
		if pool.Spec.Subnet != subnet.Spec.Subnet {
			continue
		}

		poolCopy := pool.DeepCopy()
		orphan := false
		if !metav1.IsControlledBy(poolCopy, subnet) {
			if err := ctrl.SetControllerReference(subnet, poolCopy, sc.subnetManager.runtimeMgr.GetScheme()); err != nil {
				return err
			}
			orphan = true
		}

		if poolCopy.Labels == nil {
			pool.Labels = make(map[string]string)
		}
		if v, ok := poolCopy.Labels[constant.LabelIPPoolOwnerSpiderSubnet]; !ok || v != subnet.Name {
			poolCopy.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnet.Name
			orphan = true
		}

		if orphan {
			if err := sc.subnetManager.client.Update(ctx, poolCopy); err != nil {
				return err
			}
		}
	}

	return nil
}

func (sc *SubnetController) syncControlledIPPoolIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return err
	}

	selector := labels.Set{constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name}.AsSelector()
	ipPools, err := sc.ipPoolsLister.List(selector)
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

		validIPs := spiderpoolip.IPsIntersectionSet(subnetTotalIPs, poolTotalIPs)
		tmpCount += len(validIPs)

		ranges, err := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, validIPs)
		if err != nil {
			return err
		}
		controlledIPPools[pool.Name] = spiderpoolv1.PoolIPPreAllocation{IPs: ranges}
	}
	subnet.Status.ControlledIPPools = controlledIPPools

	// Update the count of total IP addresses.
	totalIPCount := int64(len(subnetTotalIPs))
	subnet.Status.TotalIPCount = &totalIPCount

	// Update the count of pre-allocated IP addresses.
	allocatedIPCount := int64(tmpCount)
	subnet.Status.AllocatedIPCount = &allocatedIPCount

	return sc.subnetManager.client.Status().Update(ctx, subnet)
}

func (sc *SubnetController) removeFinalizer(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(subnet, metav1.FinalizerDeleteDependents) {
		if err := sc.subnetManager.client.Delete(
			ctx,
			subnet,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
		); err != nil {
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
	if err := sc.subnetManager.client.Update(ctx, subnet); err != nil {
		return err
	}
	logger.Sugar().Debugf("Remove finalizer %s", constant.SpiderFinalizer)

	return nil
}

// notifySubnetIPPool will list the subnet's corresponding auto-created IPPools,
// it will insert the IPPools name to IPPool informer work queue if the IPPool need to be scaled.
func (sc *SubnetController) notifySubnetIPPool(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)

	if sc.subnetManager.ipPoolManager.GetAutoPoolRateLimitQueue() == nil {
		logger.Warn("IPPool manager doesn't have IPPool informer rate limit workqueue!")
		return nil
	}

	selector := labels.Set{constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name}.AsSelector()
	ipPools, err := sc.ipPoolsLister.List(selector)
	if err != nil {
		return err
	}

	if len(ipPools) == 0 {
		return nil
	}

	maxQueueLength := sc.subnetManager.ipPoolManager.GetAutoPoolMaxWorkQueueLength()
	for _, pool := range ipPools {
		if pool.DeletionTimestamp != nil {
			logger.Sugar().Warnf("IPPool '%s' is terminating, no need to scale it!", pool.Name)
			continue
		}

		if ippoolmanager.ShouldScaleIPPool(pool) {
			if sc.subnetManager.ipPoolManager.GetAutoPoolRateLimitQueue().Len() >= maxQueueLength {
				logger.Sugar().Errorf("The IPPool workqueue is out of capacity, discard enqueue auto-created IPPool '%s'", pool.Name)
				return nil
			}

			sc.subnetManager.ipPoolManager.GetAutoPoolRateLimitQueue().AddRateLimited(pool.Name)
			logger.Sugar().Debugf("added '%s' to IPPool workqueue", pool.Name)
		}
	}

	return nil
}
