// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var informerLogger *zap.Logger

// SetupInformer will set up SpiderIPPool informer in circle
func (im *ipPoolManager) SetupInformer(client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to start SpiderIPPool informer, controller leader must be specified")
	}
	im.leader = controllerLeader

	informerLogger = logutils.Logger.Named("SpiderIPPool-Informer")
	informerLogger.Info("try to register SpiderIPPool informer")
	go func() {
		for {
			if !im.leader.IsElected() {
				time.Sleep(im.config.LeaderRetryElectGap)
				continue
			}

			// stopper lifecycle is same with spiderIPPool Informer
			stopper := make(chan struct{})

			go func() {
				for {
					if !im.leader.IsElected() {
						informerLogger.Error("leader lost! stop SpiderIPPool informer!")
						close(stopper)
						return
					}

					time.Sleep(im.config.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("create SpiderIPPool informer")
			factory := externalversions.NewSharedInformerFactory(client, 0)
			c := newIPPoolInformerController(im, client, factory.Spiderpool().V1().SpiderIPPools())
			factory.Start(stopper)

			if err := c.Run(im.config.WorkerNum, stopper); nil != err {
				informerLogger.Sugar().Errorf("failed to run ippool controller, error: %v", err)
			}

			informerLogger.Error("SpiderIPPool informer broken")
		}
	}()

	return nil
}

type poolInformerController struct {
	poolMgr    *ipPoolManager
	client     crdclientset.Interface
	poolLister listers.SpiderIPPoolLister
	poolSynced cache.InformerSynced

	// serves for all IPPool status process
	allPoolWorkQueue workqueue.RateLimitingInterface
	v4GenIPsCursor   bool
	v6GenIPsCursor   bool
}

func newIPPoolInformerController(poolMgr *ipPoolManager, client crdclientset.Interface, poolInformer informers.SpiderIPPoolInformer) *poolInformerController {
	c := &poolInformerController{
		poolMgr:          poolMgr,
		client:           client,
		poolLister:       poolInformer.Lister(),
		poolSynced:       poolInformer.Informer().HasSynced,
		allPoolWorkQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SpiderIPPools"),
	}

	// for all IPPool processing
	poolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAllIPPoolAdd,
		UpdateFunc: c.onAllIPPoolUpdate,
		DeleteFunc: nil,
	})

	// for auto-created IPPool processing
	if poolMgr.config.EnableSpiderSubnet {
		poolMgr.v4AutoCreatedRateLimitQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv4")
		poolMgr.v6AutoCreatedRateLimitQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv6")

		informerLogger.Debug("add auto-created IPPool processing callback hook")
		poolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueueAutoIPPool(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.enqueueAutoIPPool(newObj)
			},
			DeleteFunc: nil,
		})
	}

	return c
}

// enqueueAutoIPPool will insert auto-created IPPool name into workQueue
func (c *poolInformerController) enqueueAutoIPPool(obj interface{}) {
	pool := obj.(*spiderpoolv1.SpiderIPPool)

	if pool.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to enqueue", pool.Name)
		return
	}

	maxQueueLength := c.poolMgr.GetAutoPoolMaxWorkQueueLength()
	// only add some pools that the current IP number is not equal with the desired IP number
	if ShouldScaleIPPool(pool) {
		var workQueue workqueue.RateLimitingInterface
		ipVersion := *pool.Spec.IPVersion

		if ipVersion == constant.IPv4 {
			workQueue = c.poolMgr.v4AutoCreatedRateLimitQueue
		} else {
			workQueue = c.poolMgr.v6AutoCreatedRateLimitQueue
		}

		if workQueue.Len() >= maxQueueLength {
			informerLogger.Sugar().Errorf("The IPPool '%s' workqueue is out of capacity, discard enqueue auto-created IPPool '%s'",
				fmt.Sprintf("IPv%d", ipVersion), pool.Name)
			return
		}

		workQueue.Add(pool.Name)
		informerLogger.Sugar().Debugf("added '%s' to IPPool '%s' auto-created workqueue", pool.Name, fmt.Sprintf("IPv%d", ipVersion))
	}
}

// onAllIPPoolAdd represents SpiderIPPool informer Add Event
func (c *poolInformerController) onAllIPPoolAdd(obj interface{}) {
	pool := obj.(*spiderpoolv1.SpiderIPPool)

	err := c.updateSpiderIPPool(context.TODO(), nil, pool)
	if nil != err {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd error: %v", err)
	}
}

// onAllIPPoolUpdate represents SpiderIPPool informer Update Event
func (c *poolInformerController) onAllIPPoolUpdate(oldObj interface{}, newObj interface{}) {
	oldPool := oldObj.(*spiderpoolv1.SpiderIPPool)
	newPool := newObj.(*spiderpoolv1.SpiderIPPool)

	err := c.updateSpiderIPPool(context.TODO(), oldPool, newPool)
	if nil != err {
		informerLogger.Sugar().Errorf("onAllIPPoolUpdate error: %v", err)
	}
}

// updateSpiderIPPool serves for SpiderIPPool Informer event hooks,
// it will check whether the SpiderIPPool status AllocatedIPCount/TotalIPCount needs to be initialized
// and enqueue them.
func (c *poolInformerController) updateSpiderIPPool(ctx context.Context, oldIPPool, currentIPPool *spiderpoolv1.SpiderIPPool) error {
	// remove finalizer when SpiderIPPool object is Deleting and its Status AllocatedIPs is empty
	if currentIPPool.DeletionTimestamp != nil && len(currentIPPool.Status.AllocatedIPs) == 0 {
		err := c.poolMgr.RemoveFinalizer(ctx, currentIPPool.Name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				informerLogger.Sugar().Debugf("SpiderIPPool '%s' is already deleted", currentIPPool.Name)
				return nil
			}

			return fmt.Errorf("failed to remove SpiderIPPool '%s' finalizer '%s', error: %v", currentIPPool.Name, constant.SpiderFinalizer, err)
		}

		// no need to process it with the following function, the IPPool will be killed by kubernetes API server
		informerLogger.Sugar().Infof("remove SpiderIPPool '%s' finalizer '%s' successfully", currentIPPool.Name, constant.SpiderFinalizer)
		return nil
	}

	// update the TotalIPCount if needed
	needCalculate := false
	if currentIPPool.Status.TotalIPCount == nil || currentIPPool.Status.AllocatedIPCount == nil {
		needCalculate = true
	} else {
		if oldIPPool == nil {
			needCalculate = false
		} else {
			switch {
			case !reflect.DeepEqual(oldIPPool.Spec.IPs, currentIPPool.Spec.IPs):
				// case: SpiderIPPool spec IPs changed
				needCalculate = true

			case !reflect.DeepEqual(oldIPPool.Spec.ExcludeIPs, currentIPPool.Spec.ExcludeIPs):
				// case: SpiderIPPool spec ExcludeIPs changed
				needCalculate = true

			default:
				needCalculate = false
			}
		}
	}

	if needCalculate {
		informerLogger.Sugar().Debugf("try to add IPPool '%s' to IPPool workqueue to update its status", currentIPPool.Name)
		c.enqueueAllIPPool(currentIPPool)
	}

	return nil
}

func (c *poolInformerController) enqueueAllIPPool(pool *spiderpoolv1.SpiderIPPool) {
	if pool.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to enqueue", pool.Name)
		return
	}

	maxQueueLength := c.poolMgr.GetAutoPoolMaxWorkQueueLength()
	if c.allPoolWorkQueue.Len() >= maxQueueLength {
		informerLogger.Sugar().Errorf("The All-IPPool workqueue is out of capacity, discard enqueue IPPool '%s'", pool.Name)
		return
	}
	c.allPoolWorkQueue.Add(pool.Name)
	informerLogger.Sugar().Debugf("added '%s' to All-IPPool workqueue", pool.Name)
}

// Run will set up the event handlers for IPPool, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *poolInformerController) Run(workers int, stopCh <-chan struct{}) error {
	enableV4, enableV6 := c.poolMgr.config.EnableIPv4, c.poolMgr.config.EnableIPv6

	defer utilruntime.HandleCrash()
	defer c.allPoolWorkQueue.ShutDown()

	informerLogger.Debug("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.poolSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		informerLogger.Sugar().Debugf("Starting All IPPool processing worker '%d'", i)
		go wait.Until(c.runAllIPPoolWorker, 500*time.Millisecond, stopCh)
	}

	if c.poolMgr.config.EnableSpiderSubnet && enableV4 {
		informerLogger.Debug("Staring IPv4 Auto-created IPPool processing worker")
		defer c.poolMgr.v4AutoCreatedRateLimitQueue.ShutDown()
		go wait.Until(c.runV4AutoCreatePoolWorker, 500*time.Millisecond, stopCh)
	}
	if c.poolMgr.config.EnableSpiderSubnet && enableV6 {
		informerLogger.Debug("Staring IPv6 Auto-created IPPool processing worker")
		defer c.poolMgr.v6AutoCreatedRateLimitQueue.ShutDown()
		go wait.Until(c.runV6AutoCreatePoolWorker, 500*time.Millisecond, stopCh)
	}

	informerLogger.Info("IPPool controller workers started")

	<-stopCh
	informerLogger.Error("Shutting down IPPool controller workers")
	return nil
}

// runV4AutoCreatePoolWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// IPv4 Auto-created workQueue and try to scale it if needed
func (c *poolInformerController) runV4AutoCreatePoolWorker() {
	log := informerLogger.With(zap.String("IPPool_Informer_Worker", "IPv4_Auto_created_IPPool"))
	for c.processNextWorkItem(c.poolMgr.v4AutoCreatedRateLimitQueue, c.scaleIPPoolIfNeeded, log) {
	}
}

// runV6AutoCreatePoolWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// IPv6 Auto-created workQueue and try to scale it if needed
func (c *poolInformerController) runV6AutoCreatePoolWorker() {
	log := informerLogger.With(zap.String("IPPool_Informer_Worker", "IPv6_Auto_created_IPPool"))
	for c.processNextWorkItem(c.poolMgr.v6AutoCreatedRateLimitQueue, c.scaleIPPoolIfNeeded, log) {
	}
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// This will update SpiderIPPool status counts
func (c *poolInformerController) runAllIPPoolWorker() {
	log := informerLogger.With(zap.String("IPPool_Informer_Worker", "All_IPPool"))
	for c.processNextWorkItem(c.allPoolWorkQueue, c.updateIPPoolIPCount, log) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it with the given function handler.
// the processNextWorkItem is never invoked concurrently with the same key.
func (c *poolInformerController) processNextWorkItem(workQueue workqueue.RateLimitingInterface, fn func(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error, log *zap.Logger) bool {
	obj, shutdown := workQueue.Get()
	if shutdown {
		log.Error("workqueue is already shutdown!")
		return false
	}

	process := func(obj interface{}) error {
		defer workQueue.Done(obj)
		poolName, ok := obj.(string)
		if !ok {
			workQueue.Forget(obj)
			log.Sugar().Errorf("expected string in workQueue but got %+v", obj)
			return nil
		}

		pool, err := c.poolLister.Get(poolName)
		if nil != err {
			// The IPPool resource may no longer exist, in which case we stop
			// processing.
			if apierrors.IsNotFound(err) {
				workQueue.Forget(obj)
				log.Sugar().Debugf("IPPool '%s' in work queue no longer exists", poolName)
				return nil
			}

			workQueue.AddRateLimited(poolName)
			return fmt.Errorf("error syncing '%s': %s, requeuing", poolName, err.Error())
		}

		err = fn(context.TODO(), pool.DeepCopy())
		if nil != err {
			// discard some wrong input items
			if errors.Is(err, constant.ErrWrongInput) {
				workQueue.Forget(obj)
				return fmt.Errorf("failed to process IPPool '%s', error: %v, discarding it", pool.Name, err)
			}

			if apierrors.IsConflict(err) {
				workQueue.AddRateLimited(poolName)
				log.Sugar().Warnf("encountered ippool informer update conflict '%v', retrying...", err)
				return nil
			}

			// if we set nonnegative number for the requeue delay duration, we will requeue it. otherwise we will discard it.
			if c.poolMgr.config.WorkQueueRequeueDelayDuration >= 0 {
				if workQueue.NumRequeues(obj) < c.poolMgr.config.WorkQueueMaxRetries {
					log.Sugar().Errorf("encountered ippool informer error '%v', requeue it after '%v'", err, c.poolMgr.config.WorkQueueRequeueDelayDuration)
					workQueue.AddAfter(poolName, c.poolMgr.config.WorkQueueRequeueDelayDuration)
					return nil
				}

				log.Sugar().Warnf("out of work queue max retries, drop IPPool '%s'", pool.Name)
			}

			workQueue.Forget(obj)
			return fmt.Errorf("error syncing '%s': %s, discarding it", poolName, err.Error())
		}

		workQueue.Forget(obj)
		return nil
	}

	if err := process(obj); nil != err {
		log.Error(err.Error())
	}

	return true
}

// scaleIPPoolIfNeeded checks whether the provided SpiderIPPool needs to be scaled and try to process it.
func (c *poolInformerController) scaleIPPoolIfNeeded(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	if pool.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to scale it by subnet!", pool.Name)
		return nil
	}

	poolLabels := pool.GetLabels()
	subnetName, ok := poolLabels[constant.LabelIPPoolOwnerSpiderSubnet]
	if !ok {
		return fmt.Errorf("%w: there's no owner SpiderSubnet for IPPool '%s'", constant.ErrWrongInput, pool.Name)
	}

	if pool.Status.AutoDesiredIPCount == nil {
		informerLogger.Sugar().Debugf("maybe IPPool '%s' is just created for a while, wait for updating status DesiredIPCount", pool.Name)
		return nil
	}

	totalIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return fmt.Errorf("%w: failed to assemble Total IP addresses: %v", constant.ErrWrongInput, err)
	}

	desiredIPNum := int(*pool.Status.AutoDesiredIPCount)
	totalIPCount := len(totalIPs)

	if desiredIPNum == totalIPCount {
		// no need to scale
		return nil
	}
	informerLogger.Sugar().Debugf("IPPool '%s' need to change its IP number from '%d' to desired number '%d'", pool.Name, totalIPCount, desiredIPNum)

	if desiredIPNum > totalIPCount {
		// expand
		var cursor bool
		if *pool.Spec.IPVersion == constant.IPv4 {
			cursor = c.v4GenIPsCursor
			c.v4GenIPsCursor = !c.v4GenIPsCursor
		} else {
			cursor = c.v6GenIPsCursor
			c.v6GenIPsCursor = !c.v6GenIPsCursor
		}

		ipsFromSubnet, err := c.poolMgr.subnetManager.GenerateIPsFromSubnetWhenScaleUpIP(logutils.IntoContext(ctx, informerLogger), subnetName, pool, cursor)
		if nil != err {
			return fmt.Errorf("failed to generate IPs from subnet '%s', error: %w", subnetName, err)
		}

		informerLogger.Sugar().Infof("try to scale IPPool '%s' IP number from '%d' to '%d' with generated IPs '%v'", pool.Name, totalIPCount, desiredIPNum, ipsFromSubnet)
		// the IPPool webhook will automatically assign the scaled IP from SpiderSubnet
		err = c.poolMgr.ScaleIPPoolWithIPs(ctx, pool, ipsFromSubnet, ippoolmanagertypes.ScaleUpIP, desiredIPNum)
		if nil != err {
			return fmt.Errorf("failed to expand IPPool '%s' with IPs '%v', error: %w", pool.Name, ipsFromSubnet, err)
		}
	} else {
		// shrink: free IP number >= return IP Num
		// when it needs to scale down IP, enough IP is released to make sure it scale down successfully
		if totalIPCount-len(pool.Status.AllocatedIPs) >= totalIPCount-desiredIPNum {
			var allocatedIPRanges []string
			for tmpIP := range pool.Status.AllocatedIPs {
				allocatedIPRanges = append(allocatedIPRanges, tmpIP)
			}

			allocatedIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, allocatedIPRanges)
			if nil != err {
				return fmt.Errorf("%w: failed to parse IP ranges '%v', error: %v", constant.ErrWrongInput, allocatedIPRanges, err)
			}
			freeIPs := spiderpoolip.IPsDiffSet(totalIPs, allocatedIPs)
			discardedIPs := freeIPs[:totalIPCount-desiredIPNum]
			discardedIPRanges, err := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, discardedIPs)
			if nil != err {
				return fmt.Errorf("%w: failed to convert IPs '%v' to IP ranges, error: %v", constant.ErrWrongInput, discardedIPs, err)
			}

			informerLogger.Sugar().Infof("try to scale IPPool '%s' IP number from '%d' to '%d' with discarded IPs '%v'", pool.Name, totalIPCount, desiredIPNum, discardedIPs)
			// the IPPool webhook will automatically return the released IP back to SpiderSubnet
			err = c.poolMgr.ScaleIPPoolWithIPs(logutils.IntoContext(ctx, informerLogger), pool, discardedIPRanges, ippoolmanagertypes.ScaleDownIP, desiredIPNum)
			if nil != err {
				return fmt.Errorf("failed to shrink IPPool '%s' with IPs '%v', error: %w", pool.Name, discardedIPs, err)
			}
		}
	}

	return nil
}

// updateIPPoolIPCount will calculate and update the provided SpiderIPPool status AllocatedIPCount or TotalIPCount.
func (c *poolInformerController) updateIPPoolIPCount(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	if pool.DeletionTimestamp == nil && pool.Status.AllocatedIPCount == nil {
		pool.Status.AllocatedIPCount = pointer.Int64(0)
	}

	totalIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return fmt.Errorf("%w: failed to calculate SpiderIPPool '%s' total IP count, error: %v", constant.ErrWrongInput, pool.Name, err)
	}

	totalIPCount := int64(len(totalIPs))
	pool.Status.TotalIPCount = &totalIPCount

	err = c.poolMgr.client.Status().Update(ctx, pool)
	if nil != err {
		return err
	}

	informerLogger.Sugar().Debugf("update SpiderIPPool '%s' status TotalIPCount to '%d' successfully", pool.Name, totalIPCount)

	return nil
}
