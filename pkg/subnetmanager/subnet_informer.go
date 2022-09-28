// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
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
			factory := externalversions.NewSharedInformerFactory(client, 0)
			subnetInformer := factory.Spiderpool().V1().SpiderSubnets().Informer()
			subnetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc:    sm.onSpiderSubnetAdd,
				UpdateFunc: sm.onSpiderSubnetUpdate,
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

	ctx, cancel := context.WithCancel(sm.innerCtx)
	defer cancel()

	if err := sm.doAdd(logutils.IntoContext(ctx, logger), subnet.DeepCopy()); err != nil {
		logger.Sugar().Errorf("Failed to reconcile Subnet: %v", err)
	}
}

func (sm *subnetManager) doAdd(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	return sm.initSubnetFreeIPsAndCount(ctx, subnet)
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

	ctx, cancel := context.WithCancel(sm.innerCtx)
	defer cancel()

	if err := sm.doUpdate(logutils.IntoContext(ctx, logger), oldSubnet.DeepCopy(), newSubnet.DeepCopy()); err != nil {
		logger.Sugar().Errorf("Failed to reconcile Subnet: %v", err)
	}
}

func (sm *subnetManager) doUpdate(ctx context.Context, oldSubnet, newSubnet *spiderpoolv1.SpiderSubnet) error {
	change := false
	if !reflect.DeepEqual(newSubnet.Spec.IPs, oldSubnet.Spec.IPs) ||
		!reflect.DeepEqual(newSubnet.Spec.ExcludeIPs, oldSubnet.Spec.ExcludeIPs) {
		change = true
	}

	if change {
		return sm.initSubnetFreeIPsAndCount(ctx, newSubnet)
	}

	return nil
}

func (sm *subnetManager) initSubnetFreeIPsAndCount(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= sm.config.MaxConflictRetrys; i++ {
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

		subnetTotalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
		if err != nil {
			return err
		}
		freeIPs := subnetTotalIPs
		for _, pool := range ipPoolList.Items {
			if subnet.Spec.Subnet == pool.Spec.Subnet {
				poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
				if err != nil {
					return err
				}
				freeIPs = spiderpoolip.IPsDiffSet(freeIPs, poolTotalIPs)
			}
		}

		// Merge free IP ranges.
		ranges, err := spiderpoolip.ConvertIPsToIPRanges(*subnet.Spec.IPVersion, freeIPs)
		if err != nil {
			return err
		}
		subnet.Status.FreeIPs = ranges

		// Calculate the count of total IP addresses.
		totalIPCount := int64(len(subnetTotalIPs))
		subnet.Status.TotalIPCount = &totalIPCount

		// Calculate the count of free IP addresses.
		freeIPCount := int64(len(freeIPs))
		subnet.Status.FreeIPCount = &freeIPCount

		if err := sm.client.Status().Update(ctx, subnet); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == sm.config.MaxConflictRetrys {
				return fmt.Errorf("insufficient retries(<=%d) to init the free IP ranges of Subnet", sm.config.MaxConflictRetrys)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}
