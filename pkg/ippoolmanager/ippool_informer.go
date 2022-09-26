// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var informerLogger *zap.Logger

// SetupInformer will set up SpiderIPPool informer in circle
func (im *ipPoolManager) SetupInformer(client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to start SpiderIPPool informer, controller leader must be specified")
	}

	informerLogger = logutils.Logger.Named("SpiderIPPool-Informer")

	im.leader = controllerLeader

	informerLogger.Info("try to register SpiderIPPool informer")
	go func() {
		for {
			if !im.leader.IsElected() {
				time.Sleep(im.config.LeaderRetryElectGap)
				continue
			}

			informerLogger.Info("create SpiderIPPool informer")
			factory := externalversions.NewSharedInformerFactory(client, 0)
			spiderIPPoolInformer := factory.Spiderpool().V1().SpiderIPPools().Informer()
			spiderIPPoolInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc:    im.onSpiderIPPoolAdd,
				UpdateFunc: im.onSpiderIPPoolUpdate,
				DeleteFunc: nil,
			})
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

			spiderIPPoolInformer.Run(stopper)
			informerLogger.Error("leader lost, SpiderIPPool informer broken")
		}
	}()

	return nil
}

// onSpiderIPPoolAdd represents SpiderIPPool informer Add Event
func (im *ipPoolManager) onSpiderIPPoolAdd(obj interface{}) {
	spiderIPPool, ok := obj.(*spiderpoolv1.SpiderIPPool)
	if !ok {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd: failed to assert object '%+v' to SpiderIPPool", obj)
		return
	}

	err := im.updateSpiderIPPool(context.TODO(), nil, spiderIPPool)
	if nil != err {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd error: %v", err)
	}
}

// onSpiderIPPoolUpdate represents SpiderIPPool informer Update Event
func (im *ipPoolManager) onSpiderIPPoolUpdate(oldObj interface{}, newObj interface{}) {
	oldSpiderIPPool, ok := oldObj.(*spiderpoolv1.SpiderIPPool)
	if !ok {
		informerLogger.Sugar().Errorf("onSpiderIPPoolUpdate: failed to assert oldObj '%+v' to SpiderIPPool", oldObj)
		return
	}

	newSpiderIPPool, ok := newObj.(*spiderpoolv1.SpiderIPPool)
	if !ok {
		informerLogger.Sugar().Errorf("onSpiderIPPoolUpdate: failed to assert newObj '%+v' to SpiderIPPool", newObj)
		return
	}

	err := im.updateSpiderIPPool(context.TODO(), oldSpiderIPPool, newSpiderIPPool)
	if nil != err {
		informerLogger.Sugar().Errorf("onSpiderIPPoolAdd error: %v", err)
	}
}

// updateSpiderIPPool serves for SpiderIPPool Informer event hooks
func (im *ipPoolManager) updateSpiderIPPool(ctx context.Context, oldSpiderIPPool, currentSpiderIPPool *spiderpoolv1.SpiderIPPool) error {
	if currentSpiderIPPool == nil {
		return fmt.Errorf("currentSpiderIPPool must be specified")
	}

	// initial SpiderIPPool AllocatedIPCount property
	if currentSpiderIPPool.DeletionTimestamp == nil && currentSpiderIPPool.Status.AllocatedIPCount == nil {
		currentSpiderIPPool.Status.AllocatedIPCount = new(int64)
	}

	// remove finalizer when SpiderIPPool object is Deleting and its Status AllocatedIPs is empty
	if currentSpiderIPPool.DeletionTimestamp != nil && len(currentSpiderIPPool.Status.AllocatedIPs) == 0 {
		err := im.RemoveFinalizer(ctx, currentSpiderIPPool.Name)
		if nil != err {
			return fmt.Errorf("failed to remove SpiderIPPool '%s' finalizer '%s', error: %v", currentSpiderIPPool.Name, constant.SpiderFinalizer, err)
		}

		informerLogger.Sugar().Infof("remove SpiderIPPool '%s' finalizer '%s' successfully", currentSpiderIPPool.Name, constant.SpiderFinalizer)

		// once updated successfully the CR object resource version changed,
		// then we will meet conflict if we update other data.
		return nil
	}

	// update the TotalIPCount if needed
	needCalculate := false
	if currentSpiderIPPool.Status.TotalIPCount == nil {
		needCalculate = true
	} else {
		if oldSpiderIPPool == nil {
			needCalculate = false
		} else {
			switch {
			case !reflect.DeepEqual(oldSpiderIPPool.Spec.IPs, currentSpiderIPPool.Spec.IPs):
				// case: SpiderIPPool spec IPs changed
				needCalculate = true

			case !reflect.DeepEqual(oldSpiderIPPool.Spec.ExcludeIPs, currentSpiderIPPool.Spec.ExcludeIPs):
				// case: SpiderIPPool spec ExcludeIPs changed
				needCalculate = true

			default:
				needCalculate = false
			}
		}
	}

	if needCalculate {
		// we do a deep copy of the object here so that the caller can continue to use
		// the original object in a threadsafe manner.
		spiderIPPool := currentSpiderIPPool.DeepCopy()

		totalIPs, err := spiderpoolip.AssembleTotalIPs(*spiderIPPool.Spec.IPVersion, spiderIPPool.Spec.IPs, spiderIPPool.Spec.ExcludeIPs)
		if nil != err {
			return fmt.Errorf("failed to calculate SpiderIPPool '%s' total IP count, error: %v", currentSpiderIPPool.Name, err)
		}

		totalIPCount := int64(len(totalIPs))
		spiderIPPool.Status.TotalIPCount = &totalIPCount

		err = im.client.Status().Update(ctx, spiderIPPool)
		if nil != err {
			return fmt.Errorf("failed to update SpiderIPPool '%s' status TotalIPCount to '%d', error: %v", currentSpiderIPPool.Name, totalIPCount, err)
		}

		informerLogger.Sugar().Infof("update SpiderIPPool '%s' status TotalIPCount to '%d' successfully", currentSpiderIPPool.Name, totalIPCount)
	}

	return nil
}
