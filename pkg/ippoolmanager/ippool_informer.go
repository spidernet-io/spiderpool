// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	apitypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/event"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	subnetmanagercontrollers "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var informerLogger *zap.Logger

type IPPoolController struct {
	client client.Client

	poolLister    listers.SpiderIPPoolLister
	poolSynced    cache.InformerSynced
	subnetsLister listers.SpiderSubnetLister
	subnetsSynced cache.InformerSynced

	// serves for all IPPool status process
	allPoolWorkQueue workqueue.RateLimitingInterface

	// v4AutoCreatedRateLimitQueue serves for IPPool informer
	v4AutoCreatedRateLimitQueue workqueue.RateLimitingInterface
	v4GenIPsCursor              bool

	// v6AutoCreatedRateLimitQueue serves for IPPool informer
	v6AutoCreatedRateLimitQueue workqueue.RateLimitingInterface
	v6GenIPsCursor              bool

	IPPoolControllerConfig
}

type IPPoolControllerConfig struct {
	EnableIPv4                    bool
	EnableIPv6                    bool
	IPPoolControllerWorkers       int
	EnableSpiderSubnet            bool
	LeaderRetryElectGap           time.Duration
	MaxWorkqueueLength            int
	WorkQueueRequeueDelayDuration time.Duration
	WorkQueueMaxRetries           int
}

func NewIPPoolController(client client.Client, poolControllerConfig IPPoolControllerConfig) *IPPoolController {
	c := &IPPoolController{
		client:                 client,
		IPPoolControllerConfig: poolControllerConfig,
	}

	return c
}

func (ic *IPPoolController) SetupInformer(ctx context.Context, client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to start SpiderIPPool informer, controller leader must be specified")
	}

	informerLogger = logutils.Logger.Named("SpiderIPPool-Informer")
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
						informerLogger.Warn("Leader lost, stop Subnet informer")
						innerCancel()
						return
					}
					time.Sleep(ic.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("create SpiderIPPool informer")
			factory := externalversions.NewSharedInformerFactory(client, 0)
			ic.addEventHandlers(
				factory.Spiderpool().V1().SpiderIPPools(),
				factory.Spiderpool().V1().SpiderSubnets(),
			)
			factory.Start(innerCtx.Done())

			if err := ic.Run(innerCtx.Done()); nil != err {
				informerLogger.Sugar().Errorf("failed to run ippool controller, error: %v", err)
			}
			informerLogger.Error("SpiderIPPool informer broken")
		}
	}()

	return nil
}

func (ic *IPPoolController) addEventHandlers(poolInformer informers.SpiderIPPoolInformer, subnetInformer informers.SpiderSubnetInformer) {
	ic.poolLister = poolInformer.Lister()
	ic.poolSynced = poolInformer.Informer().HasSynced
	ic.subnetsLister = subnetInformer.Lister()
	ic.subnetsSynced = subnetInformer.Informer().HasSynced
	ic.allPoolWorkQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SpiderIPPools")

	// for all IPPool processing
	poolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ic.onAllIPPoolAdd,
		UpdateFunc: ic.onAllIPPoolUpdate,
		DeleteFunc: nil,
	})

	// for auto-created IPPool processing
	if ic.EnableSpiderSubnet {
		ic.v4AutoCreatedRateLimitQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv4")
		ic.v6AutoCreatedRateLimitQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv6")

		informerLogger.Debug("add auto-created IPPool processing callback hook")
		poolInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			// we filter out all auto-created IPPools and enqueue them
			FilterFunc: func(obj interface{}) bool {
				switch pool := obj.(type) {
				case *spiderpoolv1.SpiderIPPool:
					return IsAutoCreatedIPPool(pool)
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: ic.enqueueAutoIPPool,
				UpdateFunc: func(oldObj, newObj interface{}) {
					ic.enqueueAutoIPPool(newObj)
				},
				DeleteFunc: nil,
			},
		})

		// for all updated subnets, we need to list their corresponding auto-created IPPools,
		// it will insert the IPPools name to IPPool informer work queue if the IPPool need to be scaled.
		subnetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: ic.syncSubnetIPPools,
			UpdateFunc: func(oldObj, newObj interface{}) {
				ic.syncSubnetIPPools(newObj)
			},
			DeleteFunc: nil,
		})
	}
}

// enqueueAutoIPPool will insert auto-created IPPool name into workQueue
func (ic *IPPoolController) enqueueAutoIPPool(obj interface{}) {
	pool := obj.(*spiderpoolv1.SpiderIPPool)

	if pool.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to enqueue", pool.Name)
		return
	}

	maxQueueLength := ic.MaxWorkqueueLength
	// If the auto-created IPPool's current IP number is not equal with the desired IP number, we'll try to scale it.
	// If its allocated IPs are empty, we will check whether the IPPool should be deleted or not.
	if ShouldScaleIPPool(pool) || len(pool.Status.AllocatedIPs) == 0 {
		var workQueue workqueue.RateLimitingInterface
		ipVersion := *pool.Spec.IPVersion

		if ipVersion == constant.IPv4 {
			workQueue = ic.v4AutoCreatedRateLimitQueue
		} else {
			workQueue = ic.v6AutoCreatedRateLimitQueue
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
func (ic *IPPoolController) onAllIPPoolAdd(obj interface{}) {
	pool := obj.(*spiderpoolv1.SpiderIPPool)

	err := ic.updateSpiderIPPool(nil, pool)
	if nil != err {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd error: %v", err)
	}
}

// onAllIPPoolUpdate represents SpiderIPPool informer Update Event
func (ic *IPPoolController) onAllIPPoolUpdate(oldObj interface{}, newObj interface{}) {
	oldPool := oldObj.(*spiderpoolv1.SpiderIPPool)
	newPool := newObj.(*spiderpoolv1.SpiderIPPool)

	err := ic.updateSpiderIPPool(oldPool, newPool)
	if nil != err {
		informerLogger.Sugar().Errorf("onAllIPPoolUpdate error: %v", err)
	}
}

// updateSpiderIPPool serves for SpiderIPPool Informer event hooks,
// it will check whether the SpiderIPPool status AllocatedIPCount/TotalIPCount needs to be initialized
// and enqueue them.
func (ic *IPPoolController) updateSpiderIPPool(oldIPPool, currentIPPool *spiderpoolv1.SpiderIPPool) error {
	if currentIPPool.DeletionTimestamp != nil {
		informerLogger.Sugar().Debugf("try to add terminating IPPool '%s' to IPPool workqueue", currentIPPool.Name)
		ic.enqueueAllIPPool(currentIPPool)
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
		ic.enqueueAllIPPool(currentIPPool)
	}

	return nil
}

// enqueueAllIPPool will insert All IPPools names into workQueue
func (ic *IPPoolController) enqueueAllIPPool(pool *spiderpoolv1.SpiderIPPool) {
	if ic.allPoolWorkQueue.Len() >= ic.MaxWorkqueueLength {
		informerLogger.Sugar().Errorf("The All-IPPool workqueue is out of capacity, discard enqueue IPPool '%s'", pool.Name)
		return
	}
	ic.allPoolWorkQueue.Add(pool.Name)
	informerLogger.Sugar().Debugf("added '%s' to All-IPPool workqueue", pool.Name)
}

// Run will set up the event handlers for IPPool, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (ic *IPPoolController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer ic.allPoolWorkQueue.ShutDown()

	informerLogger.Debug("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, ic.poolSynced, ic.subnetsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < ic.IPPoolControllerWorkers; i++ {
		informerLogger.Sugar().Debugf("Starting All IPPool processing worker '%d'", i)
		go wait.Until(ic.runAllIPPoolWorker, 1*time.Second, stopCh)
	}

	if ic.EnableSpiderSubnet && ic.EnableIPv4 {
		informerLogger.Debug("Staring IPv4 Auto-created IPPool processing worker")
		defer ic.v4AutoCreatedRateLimitQueue.ShutDown()
		go wait.Until(ic.runV4AutoCreatedPoolWorker, 500*time.Millisecond, stopCh)
	}
	if ic.EnableSpiderSubnet && ic.EnableIPv6 {
		informerLogger.Debug("Staring IPv6 Auto-created IPPool processing worker")
		defer ic.v6AutoCreatedRateLimitQueue.ShutDown()
		go wait.Until(ic.runV6AutoCreatedPoolWorker, 500*time.Millisecond, stopCh)
	}

	informerLogger.Info("IPPool controller workers started")

	<-stopCh
	informerLogger.Error("Shutting down IPPool controller workers")
	return nil
}

// runV4AutoCreatePoolWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// IPv4 Auto-created workQueue and try to scale it if needed
func (ic *IPPoolController) runV4AutoCreatedPoolWorker() {
	log := informerLogger.With(zap.String("IPPool_Informer_Worker", "IPv4_Auto_created_IPPool"))
	for ic.processNextWorkItem(ic.v4AutoCreatedRateLimitQueue, ic.handleAutoCreatedIPPool, log) {
	}
}

// runV6AutoCreatePoolWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// IPv6 Auto-created workQueue and try to scale it if needed
func (ic *IPPoolController) runV6AutoCreatedPoolWorker() {
	log := informerLogger.With(zap.String("IPPool_Informer_Worker", "IPv6_Auto_created_IPPool"))
	for ic.processNextWorkItem(ic.v6AutoCreatedRateLimitQueue, ic.handleAutoCreatedIPPool, log) {
	}
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// This will update SpiderIPPool status counts
func (ic *IPPoolController) runAllIPPoolWorker() {
	log := informerLogger.With(zap.String("IPPool_Informer_Worker", "All_IPPool"))
	for ic.processNextWorkItem(ic.allPoolWorkQueue, ic.syncHandleAllIPPool, log) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it with the given function handler.
// the processNextWorkItem is never invoked concurrently with the same key.
func (ic *IPPoolController) processNextWorkItem(workQueue workqueue.RateLimitingInterface, fn func(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error, log *zap.Logger) bool {
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

		pool, err := ic.poolLister.Get(poolName)
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
			if ic.WorkQueueRequeueDelayDuration >= 0 {
				if workQueue.NumRequeues(obj) < ic.WorkQueueMaxRetries {
					log.Sugar().Errorf("encountered ippool informer error '%v', requeue it after '%v'", err, ic.WorkQueueRequeueDelayDuration)
					workQueue.AddAfter(poolName, ic.WorkQueueRequeueDelayDuration)
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

	err := process(obj)
	if nil != err {
		log.Error(err.Error())
	}

	return true
}

// scaleIPPoolIfNeeded checks whether the provided SpiderIPPool needs to be scaled and try to process it.
func (ic *IPPoolController) scaleIPPoolIfNeeded(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
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
			cursor = ic.v4GenIPsCursor
			ic.v4GenIPsCursor = !ic.v4GenIPsCursor
		} else {
			cursor = ic.v6GenIPsCursor
			ic.v6GenIPsCursor = !ic.v6GenIPsCursor
		}

		ipsFromSubnet, err := ic.generateIPsFromSubnetWhenScaleUpIP(logutils.IntoContext(ctx, informerLogger), subnetName, pool, cursor)
		if nil != err {
			return fmt.Errorf("failed to generate IPs from subnet '%s', error: %w", subnetName, err)
		}

		informerLogger.Sugar().Infof("try to scale IPPool '%s' IP number from '%d' to '%d' with generated IPs '%v'", pool.Name, totalIPCount, desiredIPNum, ipsFromSubnet)
		// the IPPool webhook will automatically assign the scaled IP from SpiderSubnet
		err = ic.scaleIPPoolWithIPs(ctx, pool, ipsFromSubnet, true, desiredIPNum)
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
			err = ic.scaleIPPoolWithIPs(logutils.IntoContext(ctx, informerLogger), pool, discardedIPRanges, false, desiredIPNum)
			if nil != err {
				return fmt.Errorf("failed to shrink IPPool '%s' with IPs '%v', error: %w", pool.Name, discardedIPs, err)
			}
		}
	}

	return nil
}

// cleanAutoIPPoolLegacy checks whether the given IPPool should be deleted or not, and the return params can show the IPPool is deleted or not
func (ic *IPPoolController) cleanAutoIPPoolLegacy(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) (isCleaned bool, err error) {
	if pool.DeletionTimestamp != nil {
		return true, nil
	}

	// Once an application was deleted we can make sure that the corresponding IPPool's IPs will be cleaned up because we have IP GC.
	// If the IPPool is cleaned, we'll check whether the IPPool's corresponding application is existed or not and process it.
	if len(pool.Status.AllocatedIPs) == 0 {
		poolLabels := pool.GetLabels()

		// check the label and decide to delete the IPPool or not
		isReclaim := poolLabels[constant.LabelIPPoolReclaimIPPool]
		if isReclaim != constant.True {
			return false, nil
		}

		// unpack the IPPool corresponding application type,namespace and name
		appLabelValue := poolLabels[constant.LabelIPPoolOwnerApplication]
		kind, ns, name, found := subnetmanagercontrollers.ParseAppLabelValue(appLabelValue)
		if !found {
			return false, fmt.Errorf("%w: invalid IPPool label '%s' value '%s'", constant.ErrWrongInput, constant.LabelIPPoolOwnerApplication, appLabelValue)
		}

		var object client.Object
		switch kind {
		case constant.KindDeployment:
			object = &appsv1.Deployment{}
		case constant.KindReplicaSet:
			object = &appsv1.ReplicaSet{}
		case constant.KindDaemonSet:
			object = &appsv1.DaemonSet{}
		case constant.KindStatefulSet:
			object = &appsv1.StatefulSet{}
		case constant.KindJob:
			object = &batchv1.Job{}
		case constant.KindCronJob:
			object = &batchv1.CronJob{}
		default:
			// pod and other controllers will clean up legacy ippools in IPAM
			return false, nil
			//return false, fmt.Errorf("%w: unmatched application kind '%s'", constant.ErrWrongInput, kind)
		}

		enableDelete := false

		// check the IPPool's corresponding application whether is existed or not
		informerLogger.Sugar().Debugf("try to get auto-created IPPool '%s' corresponding application '%s/%s/%s'", pool.Name, kind, ns, name)
		err = ic.client.Get(ctx, apitypes.NamespacedName{Namespace: ns, Name: name}, object)
		if nil != err {
			// if the application is no longer exist, we should delete the IPPool
			if apierrors.IsNotFound(err) {
				informerLogger.Sugar().Warnf("auto-created IPPool '%s' corresponding application '%s/%s/%s' is no longer exist, try to gc IPPool", pool.Name, kind, ns, name)
				enableDelete = true
			} else {
				return false, err
			}
		} else {
			// mismatch application UID
			if string(object.GetUID()) != poolLabels[constant.LabelIPPoolOwnerApplicationUID] {
				enableDelete = true
				informerLogger.Sugar().Warnf("auto-created IPPool '%v' mismatches application '%s/%s/%s' UID '%s', try to gc IPPool", pool, kind, ns, name, object.GetUID())
			}
		}

		if enableDelete {
			err := ic.client.Delete(ctx, pool)
			if client.IgnoreNotFound(err) != nil {
				return true, err
			}
			return true, nil
		}
	}

	return false, nil
}

func (ic *IPPoolController) handleAutoCreatedIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	isCleaned, err := ic.cleanAutoIPPoolLegacy(ctx, pool)
	if nil != err {
		return err
	}

	// there's no need to scale the IPPool if the IPPool is terminating.
	if isCleaned {
		return nil
	}

	err = ic.scaleIPPoolIfNeeded(ctx, pool)
	if nil != err {
		return err
	}

	return nil
}

// syncHandleAllIPPool will calculate and update the provided SpiderIPPool status AllocatedIPCount or TotalIPCount.
// And it will also remove finalizer once the IPPool is dying and no longer being used.
func (ic *IPPoolController) syncHandleAllIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
	if pool.DeletionTimestamp != nil {
		// remove finalizer to delete the dying IPPool when the IPPool is no longer being used
		if len(pool.Status.AllocatedIPs) == 0 {
			err := ic.removeFinalizer(ctx, pool)
			if nil != err {
				if apierrors.IsNotFound(err) {
					informerLogger.Sugar().Debugf("SpiderIPPool '%s' is already deleted", pool.Name)
					return nil
				}
				return fmt.Errorf("failed to remove SpiderIPPool '%s' finalizer: %w", pool.Name, err)
			}

			informerLogger.Sugar().Infof("remove SpiderIPPool '%s' finalizer successfully", pool.Name)
		}
	} else {
		// initial the original data
		if pool.Status.AllocatedIPCount == nil {
			pool.Status.AllocatedIPCount = pointer.Int64(0)
			informerLogger.Sugar().Infof("initial SpiderIPPool '%s' status AllocatedIPCount to 0", pool.Name)
		}

		totalIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
		if nil != err {
			return fmt.Errorf("%w: failed to calculate SpiderIPPool '%s' total IP count, error: %v", constant.ErrWrongInput, pool.Name, err)
		}

		totalIPCount := int64(len(totalIPs))
		pool.Status.TotalIPCount = pointer.Int64(totalIPCount)
		err = ic.client.Status().Update(ctx, pool)
		if nil != err {
			return err
		}

		informerLogger.Sugar().Debugf("update SpiderIPPool '%s' status TotalIPCount to '%d' successfully", pool.Name, totalIPCount)
	}

	return nil
}

// syncSubnetIPPools will enqueue all SpiderSubnet object corresponding IPPools name into workQueue
func (ic *IPPoolController) syncSubnetIPPools(obj interface{}) {
	subnet := obj.(*spiderpoolv1.SpiderSubnet)

	selector := labels.Set{constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name}.AsSelector()
	ipPools, err := ic.poolLister.List(selector)
	if nil != err {
		informerLogger.Sugar().Errorf("syncSubnetIPPools error: %v", err)
		return
	}

	if len(ipPools) == 0 {
		return
	}

	for _, pool := range ipPools {
		ic.enqueueAutoIPPool(pool)
	}
	event.EventRecorder.Event(subnet, corev1.EventTypeNormal, constant.EventReasonResyncSubnet, "Resynced successfully")
}

// removeFinalizer removes SpiderIPPool finalizer
func (ic *IPPoolController) removeFinalizer(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error {
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

// scaleIPPoolWithIPs will expand or shrink the IPPool with the given action.
// Notice: we shouldn't get retries in this method and the upper level calling function will requeue the workqueue once we return an error.
func (ic *IPPoolController) scaleIPPoolWithIPs(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, ipRanges []string, isScaleUp bool, desiredIPNum int) error {
	log := logutils.FromContext(ctx)

	var err error

	// filter out exclude IPs.
	currentIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return fmt.Errorf("failed to assemble Total IP addresses: %v", err)
	}

	if len(currentIPs) == desiredIPNum {
		log.Sugar().Debugf("IPPool '%s' already has desired IP number '%d' IPs, non need to scale it", pool.Name, desiredIPNum)
		return nil
	}

	if isScaleUp {
		pool.Spec.IPs = append(pool.Spec.IPs, ipRanges...)
		sortedIPRanges, err := spiderpoolip.MergeIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
		if nil != err {
			return fmt.Errorf("failed to merge IP ranges '%v', error: %v", pool.Spec.IPs, err)
		}

		log.With(zap.String("ScaleUpIP", fmt.Sprintf("add IPs '%v'", ipRanges))).
			Sugar().Infof("update IPPool '%s' IPs from '%v' to '%v'", pool.Name, pool.Spec.IPs, sortedIPRanges)
		pool.Spec.IPs = sortedIPRanges
	} else {
		discardedIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, ipRanges)
		if nil != err {
			return fmt.Errorf("failed to parse IP ranges '%v', error: %v", ipRanges, err)
		}

		// the original IPPool.Spec.IPs
		totalIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
		if nil != err {
			return fmt.Errorf("failed to parse IP ranges '%v', error: %v", pool.Spec.IPs, err)
		}

		sortedIPRanges, err := spiderpoolip.ConvertIPsToIPRanges(*pool.Spec.IPVersion, spiderpoolip.IPsDiffSet(totalIPs, discardedIPs))
		if nil != err {
			return fmt.Errorf("failed to convert IPs '%v' to IP ranges, error: %v", ipRanges, err)
		}

		log.With(zap.String("ScaleDownIP", fmt.Sprintf("discard IPs '%v'", ipRanges))).
			Sugar().Infof("update IPPool '%s' IPs from '%v' to '%v'", pool.Name, pool.Spec.IPs, sortedIPRanges)
		pool.Spec.IPs = sortedIPRanges
	}

	err = ic.client.Update(ctx, pool)
	if nil != err {
		return fmt.Errorf("failed to update IPPool '%s', error: %w", pool.Name, err)
	}

	return nil
}

// generateIPsFromSubnetWhenScaleUpIP will calculate the auto-created IPPool required IPs from corresponding SpiderSubnet and return it.
func (ic *IPPoolController) generateIPsFromSubnetWhenScaleUpIP(ctx context.Context, subnetName string, pool *spiderpoolv1.SpiderIPPool, cursor bool) ([]string, error) {
	log := logutils.FromContext(ctx)

	if pool.Status.AutoDesiredIPCount == nil {
		return nil, fmt.Errorf("%w: we can't generate IPs for the IPPool '%s' who doesn't have Status AutoDesiredIPCount", constant.ErrWrongInput, pool.Name)
	}

	var subnet spiderpoolv1.SpiderSubnet
	if err := ic.client.Get(ctx, apitypes.NamespacedName{Name: subnetName}, &subnet); err != nil {
		return nil, err
	}
	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't generate IPs from it", constant.ErrWrongInput, subnet.Name)
	}

	var ipVersion types.IPVersion
	if subnet.Spec.IPVersion != nil {
		ipVersion = *subnet.Spec.IPVersion
	} else {
		return nil, fmt.Errorf("%w: SpiderSubnet '%v' misses spec IP version", constant.ErrWrongInput, subnet)
	}

	var beforeAllocatedIPs []net.IP

	desiredIPNum := int(*pool.Status.AutoDesiredIPCount)
	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(ipVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return nil, fmt.Errorf("%w: failed to assemble IPPool '%s' total IPs, error: %v", constant.ErrWrongInput, pool.Name, err)
	}
	ipNum := desiredIPNum - len(poolTotalIPs)
	if ipNum <= 0 {
		return nil, fmt.Errorf("%w: IPPool '%s' status desiredIPNum is '%d' and total IP counts is '%d', we can't generate IPs for it",
			constant.ErrWrongInput, pool.Name, desiredIPNum, len(poolTotalIPs))
	}

	subnetPoolAllocation, ok := subnet.Status.ControlledIPPools[pool.Name]
	if ok {
		subnetPoolAllocatedIPs, err := spiderpoolip.ParseIPRanges(ipVersion, subnetPoolAllocation.IPs)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse SpiderSubnet '%s' Status ControlledIPPool '%s' IPs '%v', error: %v",
				constant.ErrWrongInput, subnet.Name, pool.Name, subnetPoolAllocation.IPs, err)
		}

		// the subnetPoolAllocatedIPs is greater than pool total IP counts indicates that
		// the SpiderSubnet updated successfully but the IPPool failed to update in the last procession
		if len(subnetPoolAllocatedIPs) > len(poolTotalIPs) {
			lastAllocatedIPs := spiderpoolip.IPsDiffSet(subnetPoolAllocatedIPs, poolTotalIPs)
			log.Sugar().Warnf("SpiderSubnet '%s' Status ControlledIPPool '%s' has the allocated IPs '%v', try to re-use it!", subnetName, pool.Name, lastAllocatedIPs)
			if len(lastAllocatedIPs) == desiredIPNum-len(poolTotalIPs) {
				// last allocated IPs is same with the current allocation request
				return spiderpoolip.ConvertIPsToIPRanges(ipVersion, lastAllocatedIPs)
			} else if len(lastAllocatedIPs) > desiredIPNum-len(poolTotalIPs) {
				// last allocated IPs is greater than the current allocation request,
				// we will update the SpiderSubnet status correctly in next IPPool webhook SpiderSubnet update procession
				return spiderpoolip.ConvertIPsToIPRanges(ipVersion, lastAllocatedIPs[:desiredIPNum-len(poolTotalIPs)])
			} else {
				// last allocated IPs less than the current allocation request,
				// we can re-use the allocated IPs and generate some another IPs
				beforeAllocatedIPs = lastAllocatedIPs
				ipNum = desiredIPNum - len(poolTotalIPs) - len(lastAllocatedIPs)
			}
		}
	}

	freeIPs, err := subnetmanagercontrollers.GenSubnetFreeIPs(&subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to generate SpiderSubnet '%s' free IPs, error: %v", subnetName, err)
	}

	// filter reserved IPs
	var reservedIPList spiderpoolv1.SpiderReservedIPList
	if err := ic.client.List(ctx, &reservedIPList); err != nil {
		return nil, fmt.Errorf("failed to list reservedIPs, error: %v", err)
	}

	reservedIPs, err := reservedipmanager.AssembleReservedIPs(ipVersion, &reservedIPList)
	if nil != err {
		return nil, fmt.Errorf("%w: failed to filter reservedIPs '%v' by IP version '%d', error: %v",
			constant.ErrWrongInput, reservedIPs, ipVersion, err)
	}

	if len(reservedIPs) != 0 {
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, reservedIPs)
	}

	if len(pool.Spec.ExcludeIPs) != 0 {
		excludeIPs, err := spiderpoolip.ParseIPRanges(ipVersion, pool.Spec.ExcludeIPs)
		if nil != err {
			return nil, fmt.Errorf("failed to parse exclude IP ranges '%v', error: %v", pool.Spec.ExcludeIPs, err)
		}
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, excludeIPs)
	}

	// check the filtered subnet free IP number is enough or not
	if len(freeIPs) < ipNum {
		return nil, fmt.Errorf("insufficient subnet FreeIPs, required '%d' but only left '%d'", ipNum, len(freeIPs))
	}

	allocateIPs := make([]net.IP, 0, ipNum)
	if cursor {
		allocateIPs = append(allocateIPs, freeIPs[:ipNum]...)
	} else {
		allocateIPs = append(allocateIPs, freeIPs[len(freeIPs)-ipNum:]...)
	}

	// re-use the last allocated IPs
	if len(beforeAllocatedIPs) != 0 {
		allocateIPs = append(allocateIPs, beforeAllocatedIPs...)
	}

	allocateIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, allocateIPs)
	if nil != err {
		return nil, err
	}

	log.Sugar().Infof("generated '%d' IPs '%v' from SpiderSubnet '%s'", ipNum, allocateIPRange, subnet.Name)

	return allocateIPRange, nil
}
