// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelapi "go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v2beta1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

const (
	messageEnqueueSubnet = "Enqueue Subnet"
	messageWorkqueueFull = "Workqueue is full, dropping the element"
)

var InformerLogger *zap.Logger

type SubnetController struct {
	Client    client.Client
	APIReader client.Reader

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

	DynamicClient    dynamic.Interface
	dynamicFactory   dynamicinformer.DynamicSharedInformerFactory
	dynamicWorkqueue workqueue.RateLimitingInterface
	recordedResource sync.Map
}

type thirdControllerKey struct {
	MetaNamespaceKey string
	AppUID           types.UID
}

func (sc *SubnetController) SetupInformer(ctx context.Context, client clientset.Interface, leader election.SpiderLeaseElector) error {
	if client == nil {
		return fmt.Errorf("spiderpoolv2beta1 clientset %w", constant.ErrMissingRequiredParam)
	}
	if leader == nil {
		return fmt.Errorf("controller leader %w", constant.ErrMissingRequiredParam)
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

			InformerLogger.Info("Initialize Dynamic informer")
			sc.dynamicFactory = dynamicinformer.NewDynamicSharedInformerFactory(sc.DynamicClient, 0)
			sc.dynamicWorkqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Dynamic-Objects")

			InformerLogger.Info("Initialize Subnet informer")
			informerFactory := externalversions.NewSharedInformerFactory(client, sc.ResyncPeriod)
			err := sc.addEventHandlers(
				informerFactory.Spiderpool().V2beta1().SpiderSubnets(),
				informerFactory.Spiderpool().V2beta1().SpiderIPPools(),
			)
			if nil != err {
				InformerLogger.Error(err.Error())
				continue
			}

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

func (sc *SubnetController) addEventHandlers(subnetInformer informers.SpiderSubnetInformer, ipPoolInformer informers.SpiderIPPoolInformer) error {
	sc.SubnetsLister = subnetInformer.Lister()
	sc.IPPoolsLister = ipPoolInformer.Lister()
	sc.SubnetIndexer = subnetInformer.Informer().GetIndexer()
	sc.IPPoolIndexer = ipPoolInformer.Informer().GetIndexer()
	sc.SubnetsSynced = subnetInformer.Informer().HasSynced
	sc.IPPoolsSynced = ipPoolInformer.Informer().HasSynced
	sc.Workqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), constant.KindSpiderSubnet)

	_, err := subnetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sc.enqueueSubnetOnAdd,
		UpdateFunc: sc.enqueueSubnetOnUpdate,
		DeleteFunc: nil,
	})
	if nil != err {
		return err
	}

	_, err = ipPoolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: sc.enqueueSubnetOnIPPoolChange,
		UpdateFunc: func(old, new interface{}) {
			oldIPPool := old.(*spiderpoolv2beta1.SpiderIPPool)
			newIPPool := new.(*spiderpoolv2beta1.SpiderIPPool)
			if reflect.DeepEqual(newIPPool.Spec.IPs, oldIPPool.Spec.IPs) &&
				reflect.DeepEqual(newIPPool.Spec.ExcludeIPs, oldIPPool.Spec.ExcludeIPs) {
				return
			}
			sc.enqueueSubnetOnIPPoolChange(new)
		},
		DeleteFunc: sc.enqueueSubnetOnIPPoolChange,
	})
	if nil != err {
		return err
	}

	return nil
}

func (sc *SubnetController) enqueueSubnetOnAdd(obj interface{}) {
	subnet := obj.(*spiderpoolv2beta1.SpiderSubnet)
	logger := InformerLogger.With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "ADD"),
	)

	if sc.Workqueue.Len() >= sc.MaxWorkqueueLength {
		logger.Sugar().Errorf(messageWorkqueueFull)
		return
	}

	sc.Workqueue.Add(subnet.Name)
	logger.Debug(messageEnqueueSubnet)
}

func (sc *SubnetController) enqueueSubnetOnUpdate(oldObj, newObj interface{}) {
	newSubnet := newObj.(*spiderpoolv2beta1.SpiderSubnet)
	logger := InformerLogger.With(
		zap.String("SubnetName", newSubnet.Name),
		zap.String("Operation", "UPDATE"),
	)

	if sc.Workqueue.Len() >= sc.MaxWorkqueueLength {
		logger.Sugar().Errorf(messageWorkqueueFull)
		return
	}

	sc.Workqueue.Add(newSubnet.Name)
	logger.Debug(messageEnqueueSubnet)
}

// enqueueSubnetOnIPPoolChange receives the IPPool resources events
func (sc *SubnetController) enqueueSubnetOnIPPoolChange(obj interface{}) {
	ipPool := obj.(*spiderpoolv2beta1.SpiderIPPool)
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
		logger.Sugar().Errorf(messageWorkqueueFull)
		return
	}

	sc.Workqueue.Add(ownerSubnet)
	logger.Debug(messageEnqueueSubnet)
}

func (sc *SubnetController) run(ctx context.Context, workers int) error {
	defer sc.Workqueue.ShutDown()
	defer sc.dynamicWorkqueue.ShutDown()

	logger := logutils.FromContext(ctx)
	logger.Info("Starting Subnet informer")

	logger.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForNamedCacheSync(constant.KindSpiderSubnet, ctx.Done(), sc.SubnetsSynced, sc.IPPoolsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, sc.runWorker, time.Second)
	}

	logger.Info("Starting dynamic informer worker")
	go wait.UntilWithContext(ctx, sc.runDynamicWorker, time.Second)

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
	logger.Info("Succeed to SYNC")

	sc.Workqueue.Forget(obj)

	return true
}

func (sc *SubnetController) runDynamicWorker(ctx context.Context) {
	for sc.processDynamicNextWorkItem(ctx) {
	}
}

func (sc *SubnetController) processDynamicNextWorkItem(ctx context.Context) bool {
	obj, shutdown := sc.dynamicWorkqueue.Get()
	if shutdown {
		return false
	}

	log := logutils.FromContext(ctx)
	process := func(obj interface{}) error {
		defer sc.dynamicWorkqueue.Done(obj)

		key, ok := obj.(thirdControllerKey)
		if !ok {
			sc.dynamicWorkqueue.Forget(obj)
			return fmt.Errorf("expected thirdControllerKey in workQueue but got '%+v'", obj)
		}

		err := sc.Client.DeleteAllOf(ctx, &spiderpoolv2beta1.SpiderIPPool{}, client.MatchingLabels{
			constant.LabelIPPoolOwnerApplicationUID: string(key.AppUID),
			constant.LabelIPPoolReclaimIPPool:       constant.True,
		})
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("third-party controller %+v corresponding auto-created IPPools already deleted", key)
				sc.dynamicWorkqueue.Forget(obj)
				return nil
			}

			sc.dynamicWorkqueue.AddRateLimited(obj)
			return fmt.Errorf("error deleting third-party controller %+v corresponding auto-created IPPools: %w, requeuing", key, err)
		}

		sc.dynamicWorkqueue.Forget(obj)
		log.Sugar().Infof("delete third-party controller %+v corresponding auto-created IPPools successfully", key)
		return nil
	}

	err := process(obj)
	if nil != err {
		log.Error(err.Error())
	}

	return true
}

func (sc *SubnetController) syncHandler(ctx context.Context, subnetName string) (err error) {
	subnet, err := sc.SubnetsLister.Get(subnetName)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	subnetCopy := subnet.DeepCopy()

	if err := sc.syncMetadata(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync metadata of Subnet: %v", err)
	}

	if err := sc.syncControllerSubnet(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync reference for controller Subnet: %v", err)
	}

	if err := sc.syncControlledIPPoolIPs(ctx, subnetCopy); err != nil {
		return fmt.Errorf("failed to sync the IP ranges of controlled IPPools of Subnet: %v", err)
	}

	if subnet.DeletionTimestamp != nil {
		if err := sc.removeFinalizer(ctx, subnetCopy); err != nil {
			return fmt.Errorf("failed to remove finalizer: %v", err)
		}
	}

	if subnetCopy.Status.TotalIPCount != nil {
		attr := attribute.String(constant.KindSpiderSubnet, subnetName)
		metric.SubnetTotalIPCounts.Add(ctx, *subnetCopy.Status.TotalIPCount, otelapi.WithAttributes(attr))
		if subnetCopy.Status.AllocatedIPCount != nil {
			metric.SubnetAvailableIPCounts.Add(ctx, (*subnetCopy.Status.TotalIPCount)-(*subnetCopy.Status.AllocatedIPCount), otelapi.WithAttributes(attr))
		} else {
			metric.SubnetAvailableIPCounts.Add(ctx, *subnetCopy.Status.TotalIPCount, otelapi.WithAttributes(attr))
		}
	}

	return nil
}

// syncMetadata add "ipam.spidernet.io/subnet-cidr" label for the SpiderSubnet object
func (sc *SubnetController) syncMetadata(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) error {
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
		return sc.Client.Update(ctx, subnet)
	}

	return nil
}

// syncControllerSubnet would set ownerReference and add "ipam.spidernet.io/owner-spider-subnet" label for the previous orphan IPPool
func (sc *SubnetController) syncControllerSubnet(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) error {
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
			if err := ctrl.SetControllerReference(subnet, poolCopy, sc.Client.Scheme()); err != nil {
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
			ippoolmanager.InheritSubnetProperties(subnet, poolCopy)
			if err := sc.Client.Update(ctx, poolCopy); err != nil {
				return err
			}
		}
	}

	return nil
}

func (sc *SubnetController) syncControlledIPPoolIPs(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)

	preAllocations, err := convert.UnmarshalSubnetAllocatedIPPools(subnet.Status.ControlledIPPools)
	if err != nil {
		return fmt.Errorf("failed to unmarshal the controlled IPPools of Subnet %s: %v", subnet.Name, err)
	}

	// Merge pre-allocated IP addresses of each IPPool and calculate their count.
	var tmpCount int
	newPreAllocations := spiderpoolv2beta1.PoolIPPreAllocations{}
	for poolName, preAllocation := range preAllocations {
		// Only auto-created IPPools have the field 'Application'.
		if preAllocation.Application != nil {
			appNamespacedName, isMatch := applicationinformers.ParseApplicationNamespacedName(*preAllocation.Application)
			if !isMatch {
				logger.Sugar().Errorf("Invalid application record %s of IPPool %s, remove the pre-allocation from Subnet", *preAllocation.Application, poolName)
				// discard this invalid allocation for subnet.status
				continue
			}

			exist, _, err := applicationinformers.IsAppExist(ctx, sc.Client, sc.DynamicClient, appNamespacedName)
			if err != nil {
				return fmt.Errorf("failed to check whether the application %v corresponding to the auto-created IPPool %s exists: %w", appNamespacedName, poolName, err)
			}

			if exist {
				// if it's a third-party controller, we'll watch its deletion hook function to GC SpiderSubnet.status
				if applicationinformers.IsThirdController(appNamespacedName) {
					err := sc.monitorThirdController(ctx, appNamespacedName)
					if nil != err {
						return fmt.Errorf("failed to monitor third-party application %v for auto-created IPPool %s: %w", appNamespacedName, poolName, err)
					}
				}
			} else {
				_, err := sc.IPPoolsLister.Get(poolName)
				if apierrors.IsNotFound(err) {
					logger.Sugar().Infof("The Application %v corresponding to the auto-created Subnet %s no longer exists, remove the pre-allocation from CIDR", appNamespacedName, poolName)
					// discard the legacy allocation for subnet.status
					continue
				}
			}

			autoIPPoolIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, preAllocation.IPs)
			if err != nil {
				logger.Sugar().Errorf("Invalid IP ranges of IPPool %s, remove the pre-allocation from Subnet", poolName)
				// discard this invalid allocation
				continue
			}
			tmpCount += len(autoIPPoolIPs)
			newPreAllocations[poolName] = preAllocation
		}
	}

	ipPools, err := sc.IPPoolsLister.List(labels.Set{constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name}.AsSelector())
	if err != nil {
		return err
	}

	// record the metric of how many IPPools the Subnet has.
	metric.SubnetPoolCounts.Record(int64(len(ipPools)), attribute.String(constant.KindSpiderSubnet, subnet.Name))

	subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return err
	}

	for _, ipPool := range ipPools {
		if !ippoolmanager.IsAutoCreatedIPPool(ipPool) {
			poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, ipPool.Spec.IPs, ipPool.Spec.ExcludeIPs)
			if err != nil {
				logger.Sugar().Errorf("Invalid total IP ranges of IPPool %s, remove the pre-allocation from Subnet", ipPool.Name)
				continue
			}
			if len(poolTotalIPs) == 0 {
				continue
			}

			validIPs := spiderpoolip.IPsIntersectionSet(subnetTotalIPs, poolTotalIPs, false)
			tmpCount += len(validIPs)

			ranges, _ := spiderpoolip.ConvertIPsToIPRanges(*ipPool.Spec.IPVersion, validIPs)
			newPreAllocations[ipPool.Name] = spiderpoolv2beta1.PoolIPPreAllocation{IPs: ranges}
		}
	}

	sync := false
	if !reflect.DeepEqual(newPreAllocations, preAllocations) {
		data, err := convert.MarshalSubnetAllocatedIPPools(newPreAllocations)
		if err != nil {
			return err
		}
		subnet.Status.ControlledIPPools = data
		allocatedIPCount := int64(tmpCount)
		subnet.Status.AllocatedIPCount = &allocatedIPCount
		sync = true
	}

	// Update the count of total IP addresses.
	totalIPCount := int64(len(subnetTotalIPs))
	if !reflect.DeepEqual(&totalIPCount, subnet.Status.TotalIPCount) {
		subnet.Status.TotalIPCount = &totalIPCount
		sync = true
	}

	if sync {
		return sc.Client.Status().Update(ctx, subnet)
	}

	return nil
}

func (sc *SubnetController) removeFinalizer(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(subnet, metav1.FinalizerDeleteDependents) {
		if err := sc.Client.Delete(
			ctx,
			subnet,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
		); err != nil {
			return err
		}

		if err := sc.APIReader.Get(ctx, types.NamespacedName{Name: subnet.Name}, subnet); err != nil {
			return err
		}
	}

	if !controllerutil.ContainsFinalizer(subnet, constant.SpiderFinalizer) {
		return nil
	}

	// Some IP addresses are still occupied by the controlled IPPools, ignore
	// to remove the finalizer.
	if subnet.Status.ControlledIPPools != nil {
		return nil
	}

	controllerutil.RemoveFinalizer(subnet, constant.SpiderFinalizer)
	if err := sc.Client.Update(ctx, subnet); err != nil {
		return err
	}
	logger.Sugar().Infof("Remove finalizer %s", constant.SpiderFinalizer)

	return nil
}

func (sc *SubnetController) monitorThirdController(ctx context.Context, appNamespacedName spiderpooltypes.AppNamespacedName) error {
	log := logutils.FromContext(ctx)

	gvr, err := applicationinformers.GenerateGVR(appNamespacedName)
	if nil != err {
		return err
	}

	_, ok := sc.recordedResource.Load(gvr)
	if !ok {
		log.Sugar().Infof("try to watch third-party controller %v delete hook for corresponding auto-created IPPools", appNamespacedName)
		informer := sc.dynamicFactory.ForResource(gvr).Informer()
		_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if nil != err {
					log.Sugar().Errorf("failed to parse object '%+v' meta key", obj)
					return
				}
				log.Sugar().Infof("received third-party controller %s delete event, enqueue it", key)
				sc.dynamicWorkqueue.Add(thirdControllerKey{
					MetaNamespaceKey: key,
					AppUID:           obj.(*unstructured.Unstructured).GetUID(),
				})
			},
		})
		if nil != err {
			return err
		}

		sc.recordedResource.Store(gvr, struct{}{})
		go func() {
			log.Sugar().Debugf("start third-party controller %v informer", appNamespacedName)
			informer.Run(ctx.Done())

			// if stopped, let's clean up the cache directly
			log.Sugar().Errorf("the third-party controller %v informer stopped")
			sc.recordedResource.Delete(gvr)
		}()
	}

	return nil
}
