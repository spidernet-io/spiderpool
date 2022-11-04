// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
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

const DefaultRetryNum = 5

var informerLogger *zap.Logger

// SetupInformer will set up SpiderIPPool informer in circle
func (im *ipPoolManager) SetupInformer(client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to start SpiderIPPool informer, controller leader must be specified")
	}

	informerLogger = logutils.Logger.Named("SpiderIPPool-Informer")

	im.leader = controllerLeader
	im.rateLimitQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SpiderIPPools")

	informerLogger.Info("try to register SpiderIPPool informer")
	go func() {
		for {
			if !im.leader.IsElected() {
				time.Sleep(im.config.LeaderRetryElectGap)
				continue
			}

			// stopper lifecycle is same with spiderIPPoolInformer
			stopper := make(chan struct{})

			go func() {
				for {
					if !im.leader.IsElected() {
						informerLogger.Warn("leader lost! stop SpiderIPPool informer!")
						close(stopper)
						return
					}

					time.Sleep(im.config.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("create SpiderIPPool informer")
			factory := externalversions.NewSharedInformerFactory(client, 0)
			c := newIPPoolInformerController(im, client, factory.Spiderpool().V1().SpiderIPPools(), im.config.EnableSpiderSubnet,
				im.rateLimitQueue, im.config.WorkQueueRequeueDelayDuration)
			factory.Start(stopper)

			if err := c.Run(1, stopper); nil != err {
				informerLogger.Sugar().Errorf("failed to run ippool controller, error: %v", err)
			}

			informerLogger.Error("leader lost, SpiderIPPool informer broken")
		}
	}()

	return nil
}

type poolInformerController struct {
	poolMgr *ipPoolManager
	client  crdclientset.Interface

	poolLister listers.SpiderIPPoolLister
	poolSynced cache.InformerSynced
	workQueue  workqueue.RateLimitingInterface

	requeueDelayDuration time.Duration
}

func newIPPoolInformerController(poolMgr *ipPoolManager, client crdclientset.Interface, poolInformer informers.SpiderIPPoolInformer,
	enableSubnet bool, rateLimitQueue workqueue.RateLimitingInterface, requeueDelayDuration time.Duration) *poolInformerController {
	c := &poolInformerController{
		poolMgr:              poolMgr,
		client:               client,
		poolLister:           poolInformer.Lister(),
		poolSynced:           poolInformer.Informer().HasSynced,
		workQueue:            rateLimitQueue,
		requeueDelayDuration: requeueDelayDuration,
	}

	// for all IPPool processing
	poolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAllIPPoolAdd,
		UpdateFunc: c.onAllIPPoolUpdate,
		DeleteFunc: nil,
	})

	// for auto-created IPPool processing
	if enableSubnet {
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

// onAllIPPoolAdd represents SpiderIPPool informer Add Event
func (c *poolInformerController) onAllIPPoolAdd(obj interface{}) {
	pool := obj.(*spiderpoolv1.SpiderIPPool)

	err := c.updateAllSpiderIPPool(context.TODO(), nil, pool)
	if nil != err {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd error: %v", err)
	}
}

// onAllIPPoolUpdate represents SpiderIPPool informer Update Event
func (c *poolInformerController) onAllIPPoolUpdate(oldObj interface{}, newObj interface{}) {
	oldPool := oldObj.(*spiderpoolv1.SpiderIPPool)
	newPool := newObj.(*spiderpoolv1.SpiderIPPool)

	err := c.updateAllSpiderIPPool(context.TODO(), oldPool, newPool)
	if nil != err {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd error: %v", err)
	}
}

// updateSpiderIPPool serves for SpiderIPPool Informer event hooks
func (c *poolInformerController) updateAllSpiderIPPool(ctx context.Context, oldIPPool, currentIPPool *spiderpoolv1.SpiderIPPool) error {
	if currentIPPool == nil {
		return fmt.Errorf("currentSpiderIPPool must be specified")
	}

	// remove finalizer when SpiderIPPool object is Deleting and its Status AllocatedIPs is empty
	if currentIPPool.DeletionTimestamp != nil && len(currentIPPool.Status.AllocatedIPs) == 0 {
		err := c.poolMgr.RemoveFinalizer(ctx, currentIPPool.Name)
		if nil != err {
			// if the IPPool object is already deleted, there's no need for us to process it at all
			if apierrors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("failed to remove SpiderIPPool '%s' finalizer '%s', error: %v", currentIPPool.Name, constant.SpiderFinalizer, err)
		}

		informerLogger.Sugar().Infof("remove SpiderIPPool '%s' finalizer '%s' successfully", currentIPPool.Name, constant.SpiderFinalizer)
	}

	// update the TotalIPCount if needed
	needCalculate := false
	if currentIPPool.Status.TotalIPCount == nil {
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
		informerLogger.Sugar().Debugf("try to update IPPool '%s' status TotalIPCount", currentIPPool.Name)
		// we do a deep copy of the object here so that the caller can continue to use
		// the original object in a threadsafe manner.
		err := c.updateIPPoolStatusCounts(ctx, currentIPPool.DeepCopy())
		if nil != err {
			return fmt.Errorf("failed to update IPPool '%s' status TotalIPCount, error: %v", currentIPPool.Name, err)
		}
	}

	return nil
}

func (c *poolInformerController) updateIPPoolStatusCounts(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	rand.Seed(time.Now().UnixNano())

	deepCopyPool := pool.DeepCopy()
	for i := 0; i <= c.poolMgr.config.MaxConflictRetries; i++ {
		var err error
		if i != 0 {
			deepCopyPool, err = c.poolMgr.GetIPPoolByName(ctx, deepCopyPool.Name)
			if nil != err {
				return err
			}
		}

		// initial SpiderIPPool AllocatedIPCount property
		if deepCopyPool.DeletionTimestamp == nil && deepCopyPool.Status.AllocatedIPCount == nil {
			deepCopyPool.Status.AllocatedIPCount = pointer.Int64(0)
		}

		totalIPs, err := spiderpoolip.AssembleTotalIPs(*deepCopyPool.Spec.IPVersion, deepCopyPool.Spec.IPs, deepCopyPool.Spec.ExcludeIPs)
		if nil != err {
			return fmt.Errorf("failed to calculate SpiderIPPool '%s' total IP count, error: %v", deepCopyPool.Name, err)
		}

		totalIPCount := int64(len(totalIPs))
		deepCopyPool.Status.TotalIPCount = &totalIPCount

		err = c.poolMgr.client.Status().Update(ctx, deepCopyPool)
		if nil != err {
			if !apierrors.IsConflict(err) {
				return err
			}

			if i == c.poolMgr.config.MaxConflictRetries {
				return fmt.Errorf("%w, failed for %d times, failed to initialize the free IP ranges of Subnet", constant.ErrRetriesExhausted, c.poolMgr.config.MaxConflictRetries)
			}

			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * c.poolMgr.config.ConflictRetryUnitTime)
			continue
		}

		pool = deepCopyPool
		informerLogger.Sugar().Debugf("update SpiderIPPool '%s' status TotalIPCount to '%d' successfully", deepCopyPool.Name, totalIPCount)
		break
	}

	return nil
}

// enqueueAutoIPPool will insert auto-created IPPool name into workQueue
func (c *poolInformerController) enqueueAutoIPPool(obj interface{}) {
	pool := obj.(*spiderpoolv1.SpiderIPPool)

	if pool.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to scale it!", pool.Name)
		return
	}

	maxQueueLength := c.poolMgr.GetAutoPoolMaxWorkQueueLength()
	// only add some pools that the current IP number is not equal with the desired IP number
	if ShouldScaleIPPool(*pool) {
		if c.workQueue.Len() >= maxQueueLength {
			informerLogger.Sugar().Errorf("The IPPool workqueue is out of capacity, discard enqueue auto-created IPPool '%s'", pool.Name)
			return
		}

		c.workQueue.Add(pool.Name)
		informerLogger.Sugar().Debugf("added '%s' to IPPool workqueue", pool.Name)
	}
}

// Run will set up the event handlers for IPPool, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *poolInformerController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workQueue.ShutDown()

	informerLogger.Debug("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.poolSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	informerLogger.Info("IPPool controller workers started")

	<-stopCh
	informerLogger.Warn("Shutting down IPPool controller workers")
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workQueue.
func (c *poolInformerController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to scale those IPPool which needs
func (c *poolInformerController) processNextWorkItem() bool {
	obj, shutdown := c.workQueue.Get()

	if shutdown {
		return false
	}

	// it doesn't master to discard some data,
	// the subnet informer and its resync will add those not desired pool name to the workqueue
	process := func(obj interface{}) error {
		defer c.workQueue.Done(obj)
		poolName, ok := obj.(string)
		if !ok {
			c.workQueue.Forget(obj)
			informerLogger.Sugar().Errorf("expected string in workqueue but got %+v", obj)
			return nil
		}

		pool, err := c.poolLister.Get(poolName)
		if nil != err {
			// The IPPool resource may no longer exist, in which case we stop
			// processing.
			if apierrors.IsNotFound(err) {
				c.workQueue.Forget(obj)
				informerLogger.Sugar().Debugf("IPPool '%s' in work queue no longer exists", poolName)
				return nil
			}

			c.workQueue.AddRateLimited(poolName)
			return fmt.Errorf("error syncing '%s': %s, requeuing", poolName, err.Error())
		}

		err = c.scaleIPPoolIfNeeded(context.TODO(), pool)
		if nil != err {
			// discard some wrong input items
			if errors.Is(err, constant.ErrWrongInput) {
				c.workQueue.Forget(obj)
				return fmt.Errorf("failed to scale IPPool '%s', error: %v", pool.Name, err)
			}

			if apierrors.IsConflict(err) {
				c.workQueue.AddRateLimited(poolName)
				informerLogger.Sugar().Warnf("encountered update conflict '%v', retrying...", err)
				return nil
			}

			// if we set positive number for the requeue delay duration, we will requeue it. otherwise we will discard it.
			if c.requeueDelayDuration >= 0 {
				informerLogger.Sugar().Warnf("encountered error '%v', requeue it after '%v'", err, c.requeueDelayDuration)
				c.workQueue.AddAfter(poolName, c.requeueDelayDuration)
				return nil
			}

			c.workQueue.Forget(obj)
			return fmt.Errorf("error syncing '%s': %s", poolName, err.Error())
		}

		c.workQueue.Forget(obj)
		return nil
	}

	if err := process(obj); nil != err {
		informerLogger.Error(err.Error())
		return true
	}

	return true
}

func (c *poolInformerController) scaleIPPoolIfNeeded(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	if pool.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to scale it!", pool.Name)
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

	pool = pool.DeepCopy()

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
		ipsFromSubnet, err := c.poolMgr.subnetManager.GenerateIPsFromSubnetWhenScaleUpIP(logutils.IntoContext(ctx, informerLogger), subnetName, pool)
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
			// sort freeIPs
			sort.Slice(freeIPs, func(i, j int) bool {
				return bytes.Compare(freeIPs[i].To16(), freeIPs[j].To16()) < 0
			})
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
