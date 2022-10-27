// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var informerLogger *zap.Logger

func (sm *subnetManager) SetupInformer(ctx context.Context, client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to set up SpiderSubnet informer, controller leader must be specified")
	}
	// TODO(iiiceoo): ctx.Done() --> close(stopper)
	sm.innerCtx = ctx
	sm.leader = controllerLeader

	informerLogger = logutils.Logger.Named("Subnet-Informer")
	go func() {
		for {
			if !sm.leader.IsElected() {
				time.Sleep(sm.config.LeaderRetryElectGap)
				continue
			}

			informerLogger.Info("Initialize SpiderSubnet informer")
			factory := externalversions.NewSharedInformerFactory(client, sm.config.ResyncPeriod)
			subnetInformer := factory.Spiderpool().V1().SpiderSubnets().Informer()
			subnetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc:    sm.onSpiderSubnetAdd,
				UpdateFunc: sm.onSpiderSubnetUpdate,
				DeleteFunc: nil,
			})

			// concurrent callback hook to notify the subnet's corresponding auto-created IPPools to check scale
			subnetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					sm.notifySubnetIPPool(obj)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					sm.notifySubnetIPPool(newObj)
				},
				DeleteFunc: nil,
			})

			stopper := make(chan struct{})
			go func() {
				for {
					if !sm.leader.IsElected() {
						informerLogger.Warn("Leader lost, stop SpiderSubnet informer")
						close(stopper)
						return
					}

					time.Sleep(sm.config.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("Starting SpiderSubnet informer")
			subnetInformer.Run(stopper)
			informerLogger.Info("SpiderSubnet informer down")
		}
	}()

	return nil
}

func (sm *subnetManager) onSpiderSubnetAdd(obj interface{}) {
	subnet, _ := obj.(*spiderpoolv1.SpiderSubnet)
	logger := informerLogger.With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "ADD"),
	)
	logger.Sugar().Debugf("Reconcile Subnet: %+v", *subnet)

	if err := sm.reconcileOnAdd(logutils.IntoContext(sm.innerCtx, logger), subnet.DeepCopy()); err != nil {
		logger.Sugar().Errorf("Failed to reconcile Subnet: %v", err)
	}
}

func (sm *subnetManager) reconcileOnAdd(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	if err := sm.initControllerSubnet(ctx, subnet); err != nil {
		return err
	}
	if err := sm.initControlledIPPoolIPs(ctx, subnet); err != nil {
		return err
	}

	if subnet.DeletionTimestamp != nil {
		if err := sm.removeFinalizer(ctx, subnet); err != nil {
			return fmt.Errorf("failed to remove finalizer: %v", err)
		}
	}

	return nil
}

func (sm *subnetManager) onSpiderSubnetUpdate(oldObj interface{}, newObj interface{}) {
	oldSubnet, _ := oldObj.(*spiderpoolv1.SpiderSubnet)
	newSubnet, _ := newObj.(*spiderpoolv1.SpiderSubnet)
	logger := informerLogger.With(
		zap.String("SubnetName", newSubnet.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Reconcile old Subnet: %+v", *oldSubnet)
	logger.Sugar().Debugf("Reconcile new Subnet: %+v", *newSubnet)

	if err := sm.reconcileOnUpdate(logutils.IntoContext(sm.innerCtx, logger), oldSubnet.DeepCopy(), newSubnet.DeepCopy()); err != nil {
		logger.Sugar().Errorf("Failed to reconcile Subnet: %v", err)
	}
}

func (sm *subnetManager) reconcileOnUpdate(ctx context.Context, oldSubnet, newSubnet *spiderpoolv1.SpiderSubnet) error {
	if newSubnet.DeletionTimestamp != nil {
		if oldSubnet.DeletionTimestamp == nil {
			if err := sm.client.Delete(
				ctx,
				newSubnet,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			); err != nil {
				return err
			}
		}

		if err := sm.removeFinalizer(ctx, newSubnet); err != nil {
			return fmt.Errorf("failed to remove finalizer: %v", err)
		}
		return nil
	}

	totalIPsChange := false
	if !reflect.DeepEqual(newSubnet.Spec.IPs, oldSubnet.Spec.IPs) ||
		!reflect.DeepEqual(newSubnet.Spec.ExcludeIPs, oldSubnet.Spec.ExcludeIPs) {
		totalIPsChange = true
	}

	if totalIPsChange {
		if err := sm.initControllerSubnet(ctx, newSubnet); err != nil {
			return err
		}
		return sm.initControlledIPPoolIPs(ctx, newSubnet)
	}

	return nil
}

func (sm *subnetManager) initControllerSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	rand.Seed(time.Now().UnixNano())
OUTER:
	for i := 0; i <= sm.config.MaxConflictRetries; i++ {
		var err error
		if i != 0 {
			subnet, err = sm.GetSubnetByName(ctx, subnet.Name)
			if err != nil {
				return err
			}
		}

		ipPoolList, err := sm.ipPoolManager.ListIPPools(ctx)
		if err != nil {
			return err
		}

		for _, pool := range ipPoolList.Items {
			if pool.Spec.Subnet != subnet.Spec.Subnet {
				continue
			}

			orphan := false
			if !metav1.IsControlledBy(&pool, subnet) {
				if err := ctrl.SetControllerReference(subnet, &pool, sm.runtimeMgr.GetScheme()); err != nil {
					return err
				}
				orphan = true
			}

			if pool.Labels == nil {
				pool.Labels = make(map[string]string)
			}
			if v, ok := pool.Labels[constant.LabelIPPoolOwnerSpiderSubnet]; !ok || v != subnet.Name {
				pool.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnet.Name
				orphan = true
			}

			if orphan {
				if err := sm.client.Update(ctx, &pool); err != nil {
					if !apierrors.IsConflict(err) {
						return err
					}
					if i == sm.config.MaxConflictRetries {
						return fmt.Errorf("%w(<=%d) to init reference for controller Subnet", constant.ErrRetriesExhausted, sm.config.MaxConflictRetries)
					}
					time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
					continue OUTER
				}
			}
		}
		break
	}

	return nil
}

func (sm *subnetManager) initControlledIPPoolIPs(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= sm.config.MaxConflictRetries; i++ {
		var err error
		if i != 0 {
			subnet, err = sm.GetSubnetByName(ctx, subnet.Name)
			if err != nil {
				return err
			}
		}

		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name,
			},
		})
		if err != nil {
			return err
		}
		ipPoolList, err := sm.ipPoolManager.ListIPPools(ctx, client.MatchingLabelsSelector{Selector: selector})
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
		for _, pool := range ipPoolList.Items {
			if pool.Spec.Subnet == subnet.Spec.Subnet {
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
		}
		subnet.Status.ControlledIPPools = controlledIPPools

		// Update the count of total IP addresses.
		totalIPCount := int64(len(subnetTotalIPs))
		subnet.Status.TotalIPCount = &totalIPCount

		// Update the count of pre-allocated IP addresses.
		allocatedIPCount := int64(tmpCount)
		subnet.Status.AllocatedIPCount = &allocatedIPCount

		if err := sm.client.Status().Update(ctx, subnet); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == sm.config.MaxConflictRetries {
				return fmt.Errorf("%w(<=%d) to init the free IP ranges of Subnet", constant.ErrRetriesExhausted, sm.config.MaxConflictRetries)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

func (sm *subnetManager) removeFinalizer(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	for i := 0; i <= sm.config.MaxConflictRetries; i++ {
		var err error
		if i != 0 {
			subnet, err = sm.GetSubnetByName(ctx, subnet.Name)
			if err != nil {
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
		if err := sm.client.Update(ctx, subnet); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == sm.config.MaxConflictRetries {
				return fmt.Errorf("%w(<=%d) to remove finalizer '%s' from Subnet %s", constant.ErrRetriesExhausted, sm.config.MaxConflictRetries, constant.SpiderFinalizer, subnet.Name)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

// notifySubnetIPPool will list the subnet's corresponding auto-created IPPools,
// it will insert the IPPools name to IPPool informer work queue if the IPPool need to be scaled.
func (sm *subnetManager) notifySubnetIPPool(obj interface{}) {
	if sm.ipPoolManager.GetAutoPoolRateLimitQueue() == nil {
		informerLogger.Warn("IPPool manager doesn't have IPPool informer rate limit workqueue!")
		return
	}

	subnet := obj.(*spiderpoolv1.SpiderSubnet)

	if subnet.DeletionTimestamp != nil {
		informerLogger.Sugar().Warnf("SpiderSubnet '%s' is terminating, no need to scale its corresponding IPPools!", subnet.Name)
		return
	}

	matchLabel := client.MatchingLabels{
		constant.LabelIPPoolOwnerSpiderSubnet: subnet.Name,
	}
	autoPoolList, err := sm.ipPoolManager.ListIPPools(context.TODO(), matchLabel)
	if nil != err {
		informerLogger.Sugar().Errorf("failed to list IPPools with labels '%v', error: %v", matchLabel, err)
		return
	}

	if autoPoolList == nil || len(autoPoolList.Items) == 0 {
		return
	}

	maxQueueLength := sm.ipPoolManager.GetAutoPoolMaxWorkQueueLength()
	for _, pool := range autoPoolList.Items {
		if pool.DeletionTimestamp != nil {
			informerLogger.Sugar().Warnf("IPPool '%s' is terminating, no need to scale it!", pool.Name)
			continue
		}

		if ippoolmanager.ShouldScaleIPPool(pool) {
			if sm.ipPoolManager.GetAutoPoolRateLimitQueue().Len() >= maxQueueLength {
				informerLogger.Sugar().Errorf("The IPPool workqueue is out of capacity, discard enqueue auto-created IPPool '%s'", pool.Name)
				return
			}

			sm.ipPoolManager.GetAutoPoolRateLimitQueue().AddRateLimited(pool.Name)
			informerLogger.Sugar().Debugf("added '%s' to IPPool workqueue", pool.Name)
		}
	}
}
