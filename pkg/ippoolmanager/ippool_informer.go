// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelapi "go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v2beta1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var informerLogger *zap.Logger

type IPPoolController struct {
	IPPoolControllerConfig
	client        client.Client
	dynamicClient dynamic.Interface
	poolLister    listers.SpiderIPPoolLister
	poolSynced    cache.InformerSynced
	poolWorkqueue workqueue.RateLimitingInterface
}

type IPPoolControllerConfig struct {
	IPPoolControllerWorkers       int
	EnableSpiderSubnet            bool
	EnableAutoPoolForApplication  bool
	MaxWorkqueueLength            int
	WorkQueueMaxRetries           int
	LeaderRetryElectGap           time.Duration
	WorkQueueRequeueDelayDuration time.Duration
	ResyncPeriod                  time.Duration
}

func NewIPPoolController(poolControllerConfig IPPoolControllerConfig, client client.Client, dynamicClient dynamic.Interface) *IPPoolController {
	informerLogger = logutils.Logger.Named("SpiderIPPool-Informer")

	c := &IPPoolController{
		IPPoolControllerConfig: poolControllerConfig,
		client:                 client,
		dynamicClient:          dynamicClient,
	}

	return c
}

func (ic *IPPoolController) SetupInformer(ctx context.Context, client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to start SpiderIPPool informer, controller leader must be specified")
	}

	informerLogger.Info("try to register SpiderIPPool informer")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !controllerLeader.IsElected() {
				time.Sleep(ic.LeaderRetryElectGap)
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

					if !controllerLeader.IsElected() {
						informerLogger.Warn("Leader lost, stop IPPool informer")
						innerCancel()
						return
					}
					time.Sleep(ic.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("create SpiderIPPool informer")
			factory := externalversions.NewSharedInformerFactory(client, ic.ResyncPeriod)
			err := ic.addEventHandlers(factory.Spiderpool().V2beta1().SpiderIPPools())
			if nil != err {
				informerLogger.Error(err.Error())
				continue
			}
			factory.Start(innerCtx.Done())

			if err := ic.Run(innerCtx.Done()); nil != err {
				informerLogger.Sugar().Errorf("failed to run ippool controller, error: %w", err)
			}
			informerLogger.Error("SpiderIPPool informer broken")
		}
	}()

	return nil
}

func (ic *IPPoolController) addEventHandlers(poolInformer informers.SpiderIPPoolInformer) error {
	ic.poolLister = poolInformer.Lister()
	ic.poolSynced = poolInformer.Informer().HasSynced

	ic.poolWorkqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SpiderIPPools")

	// for all IPPool processing
	_, err := poolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: ic.enqueueIPPool,
		UpdateFunc: func(oldObj, newObj interface{}) {
			ic.enqueueIPPool(newObj)
		},
		DeleteFunc: func(obj interface{}) {
		},
	})
	if nil != err {
		return err
	}

	return nil
}

// enqueueIPPool will check the given pool and enqueue them into different workqueue
func (ic *IPPoolController) enqueueIPPool(obj interface{}) {
	pool := obj.(*spiderpoolv2beta1.SpiderIPPool)

	// the Normal IPPools enqueue the corresponding NormalPoolWorkqueue
	if ic.poolWorkqueue.Len() >= ic.MaxWorkqueueLength {
		informerLogger.Sugar().Errorf("The IPPool workqueue is out of capacity, discard enqueue IPPool '%s'", pool.Name)
		return
	}
	ic.poolWorkqueue.Add(pool.Name)
	informerLogger.Sugar().Debugf("added '%s' to IPPool workqueue", pool.Name)
}

// Run will set up the event handlers for IPPool, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (ic *IPPoolController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer ic.poolWorkqueue.ShutDown()

	informerLogger.Debug("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, ic.poolSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < ic.IPPoolControllerWorkers; i++ {
		informerLogger.Sugar().Debugf("Starting Normal IPPool processing worker '%d'", i)
		go wait.Until(ic.runWorker, 1*time.Second, stopCh)
	}

	<-stopCh
	informerLogger.Error("Shutting down IPPool controller workers")
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// This will update SpiderIPPool status counts
func (ic *IPPoolController) runWorker() {
	for ic.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it with the given function handler.
// the processNextWorkItem is never invoked concurrently with the same key.
func (ic *IPPoolController) processNextWorkItem() bool {
	obj, shutdown := ic.poolWorkqueue.Get()
	if shutdown {
		informerLogger.Error("IPPool workqueue is already shutdown!")
		return false
	}

	process := func(obj interface{}) error {
		defer ic.poolWorkqueue.Done(obj)
		poolName, ok := obj.(string)
		if !ok {
			ic.poolWorkqueue.Forget(obj)
			informerLogger.Sugar().Errorf("expected string in workQueue but got %+v", obj)
			return nil
		}

		pool, err := ic.poolLister.Get(poolName)
		if nil != err {
			// The IPPool resource may no longer exist, in which case we stop
			// processing.
			if apierrors.IsNotFound(err) {
				ic.poolWorkqueue.Forget(obj)
				informerLogger.Sugar().Debugf("IPPool '%s' in work queue no longer exists", poolName)
				return nil
			}

			ic.poolWorkqueue.AddRateLimited(poolName)
			return fmt.Errorf("error syncing '%s': %s, requeuing", poolName, err.Error())
		}

		err = ic.handleIPPool(context.TODO(), pool.DeepCopy())
		if nil != err {
			// discard some wrong input items
			if errors.Is(err, constant.ErrWrongInput) {
				ic.poolWorkqueue.Forget(obj)
				return fmt.Errorf("failed to process IPPool '%s', error: %w, discarding it", pool.Name, err)
			}

			if apierrors.IsConflict(err) {
				ic.poolWorkqueue.AddRateLimited(poolName)
				informerLogger.Sugar().Warnf("encountered ippool informer update conflict '%v', retrying...", err)
				return nil
			}

			// if we set nonnegative number for the requeue delay duration, we will requeue it. otherwise we will discard it.
			if ic.WorkQueueRequeueDelayDuration >= 0 {
				if ic.poolWorkqueue.NumRequeues(obj) < ic.WorkQueueMaxRetries {
					informerLogger.Sugar().Errorf("encountered ippool informer error '%v', requeue it after '%v'", err, ic.WorkQueueRequeueDelayDuration)
					ic.poolWorkqueue.AddAfter(poolName, ic.WorkQueueRequeueDelayDuration)
					return nil
				}

				informerLogger.Sugar().Warnf("out of work queue max retries, drop IPPool '%s'", pool.Name)
			}

			ic.poolWorkqueue.Forget(obj)
			return fmt.Errorf("error syncing '%s': %s, discarding it", poolName, err.Error())
		}

		ic.poolWorkqueue.Forget(obj)
		return nil
	}

	err := process(obj)
	if nil != err {
		informerLogger.Error(err.Error())
	}

	return true
}

func (ic *IPPoolController) handleIPPool(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool) (err error) {
	// checkout the Auto-created IPPools whether need to scale or clean up legacies
	if ic.EnableSpiderSubnet && ic.EnableAutoPoolForApplication && IsAutoCreatedIPPool(pool) {
		err := ic.cleanAutoIPPoolLegacy(ctx, pool)
		if nil != err {
			return err
		}
	}

	// update the IPPool status properties
	err = ic.syncHandler(ctx, pool)
	if nil != err {
		return err
	}

	// metrics
	if pool.Status.TotalIPCount != nil {
		attr := attribute.String(constant.KindSpiderIPPool, pool.Name)
		metric.IPPoolTotalIPCounts.Add(ctx, *pool.Status.TotalIPCount, otelapi.WithAttributes(attr))
		if pool.Status.AllocatedIPCount != nil {
			metric.IPPoolAvailableIPCounts.Add(ctx, (*pool.Status.TotalIPCount)-(*pool.Status.AllocatedIPCount), otelapi.WithAttributes(attr))
		} else {
			metric.IPPoolAvailableIPCounts.Add(ctx, *pool.Status.TotalIPCount, otelapi.WithAttributes(attr))
		}
	}

	return nil
}

// syncHandler will calculate and update the provided SpiderIPPool status AllocatedIPCount or TotalIPCount.
// And it will also remove finalizer once the IPPool is dying and no longer being used.
func (ic *IPPoolController) syncHandler(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool) error {
	// remove finalizer to delete the dying IPPool when the IPPool is no longer being used
	if pool.DeletionTimestamp != nil && pool.Status.AllocatedIPs == nil {
		err := ic.removeFinalizer(ctx, pool)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to remove SpiderIPPool '%s' finalizer: %w", pool.Name, err)
		}

		informerLogger.Sugar().Infof("remove SpiderIPPool '%s' finalizer successfully", pool.Name)
		return nil
	}

	needUpdate := false
	// initial the original data
	if pool.Status.AllocatedIPCount == nil {
		needUpdate = true
		pool.Status.AllocatedIPCount = ptr.To(int64(0))
		informerLogger.Sugar().Infof("initial SpiderIPPool '%s' status AllocatedIPCount to 0", pool.Name)
	}

	subnet, err := spiderpoolip.NewCIDR(pool.Spec.Subnet, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if err != nil {
		return fmt.Errorf("%w: failed to calculate SpiderIPPool '%s' total IP count, error: %w", constant.ErrWrongInput, pool.Name, err)
	}
	total := subnet.TotalIPInt()
	if pool.Status.TotalIPCount == nil || *pool.Status.TotalIPCount != int64(total) {
		needUpdate = true
		pool.Status.TotalIPCount = ptr.To(int64(total))
	}

	if needUpdate {
		err = ic.client.Status().Update(ctx, pool)
		if nil != err {
			return fmt.Errorf("failed to update pool: %w", err)
		}
		informerLogger.Sugar().Debugf("update SpiderIPPool '%s' status TotalIPCount to '%d' successfully", pool.Name, *pool.Status.TotalIPCount)
	}

	return nil
}

// removeFinalizer removes SpiderIPPool finalizer
func (ic *IPPoolController) removeFinalizer(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool) error {
	if !controllerutil.ContainsFinalizer(pool, constant.SpiderFinalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(pool, constant.SpiderFinalizer)
	err := ic.client.Update(ctx, pool)
	if nil != err {
		return err
	}

	return nil
}

// cleanAutoIPPoolLegacy checks whether the given IPPool should be deleted or not
func (ic *IPPoolController) cleanAutoIPPoolLegacy(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool) error {
	if pool.DeletionTimestamp != nil {
		return nil
	}

	poolLabels := pool.GetLabels()

	// check the label and decide to delete the IPPool or not
	isReclaim := poolLabels[constant.LabelIPPoolReclaimIPPool]
	if isReclaim != constant.True {
		return nil
	}

	if pool.Status.AllocatedIPs == nil {
		// unpack the IPPool corresponding application type,namespace and name
		appGVStr := poolLabels[constant.LabelIPPoolOwnerApplicationGV]
		appAPIVersion, isMatch := applicationinformers.ParseApplicationGVStr(appGVStr)
		if !isMatch {
			return fmt.Errorf("%w: invalid IPPool label '%s' value '%s'", constant.ErrWrongInput, constant.LabelIPPoolOwnerApplicationGV, appGVStr)
		}

		appNamespacedName := types.AppNamespacedName{
			APIVersion: appAPIVersion,
			Kind:       poolLabels[constant.LabelIPPoolOwnerApplicationKind],
			Namespace:  poolLabels[constant.LabelIPPoolOwnerApplicationNamespace],
			Name:       poolLabels[constant.LabelIPPoolOwnerApplicationName],
		}

		enableDelete := false
		// check the IPPool's corresponding application whether is existed or not
		informerLogger.Sugar().Debugf("try to get auto-created IPPool '%s' corresponding application '%v'", pool.Name, appNamespacedName)
		isAppExist, appUID, err := applicationinformers.IsAppExist(ctx, ic.client, ic.dynamicClient, appNamespacedName)
		if nil != err {
			return fmt.Errorf("failed to get auto-created IPPool '%s' corresponding application '%v': %w", pool.Name, appNamespacedName, err)
		}

		// mismatch application UID
		if !isAppExist {
			informerLogger.Sugar().Warnf("auto-created IPPool '%s' corresponding application '%v' no longer exist, try to delete IPPool", pool.Name, appNamespacedName)
			enableDelete = true
		} else {
			if string(appUID) != poolLabels[constant.LabelIPPoolOwnerApplicationUID] {
				informerLogger.Sugar().Warnf("auto-created IPPool '%s' mismatches application '%v' UID '%s', try to delete IPPool", pool.Name, appNamespacedName, appUID)
				enableDelete = true
			}
		}

		if enableDelete {
			err := ic.client.Delete(ctx, pool)
			if client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete legacy auto-created IPPool %s: %w", pool.Name, err)
			}
			return nil
		}
	}

	return nil
}
