// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

// SetupReconcile registers IPPool CR object informer
func (im *ipPoolManager) SetupReconcile(leader election.SpiderLeaseElector) error {
	if leader == nil {
		return fmt.Errorf("failed to set up IPPool reconcile, leader must be specified")
	}

	im.leader = leader

	return ctrl.NewControllerManagedBy(im.runtimeMgr).
		For(&spiderpoolv1.IPPool{}).
		Complete(im)
}

// Reconcile will watch every event that related with IPPool CR object
func (im *ipPoolManager) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// backup controller could be elected as master
	if !im.leader.IsElected() {
		return reconcile.Result{}, nil
	}

	ipPool, err := im.GetIPPoolByName(ctx, request.Name)
	if nil != err {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Debugf("reconcile found deleted IPPool '%+v'", request.NamespacedName)
		} else {
			//TODO (Icarus9913): we will meet error here, should we return it and requeue ?
			logger.Sugar().Errorf("failed to fetch IPPool '%+v', error: %v", request.NamespacedName, err)
		}

		return reconcile.Result{}, nil
	}

	// calculate IPPool subresource status TotalIPCount when the CR object isn't deleting
	// TODO (Icarus9913): maybe we could abstract here to an API in the future?
	if ipPool.DeletionTimestamp == nil {
		// get IPPool usable IPs number
		totalIPs, err := im.AssembleTotalIPs(ctx, ipPool)
		if nil != err {
			//TODO (Icarus9913): we will meet error here, should we return it and requeue ?
			logger.Sugar().Errorf("failed to calculate IPPool '%s/%s' total IP count, error: %v", request.Namespace, request.Name, err)
			return reconcile.Result{}, nil
		}

		totalIPCount := int64(len(totalIPs))

		// check the IPPool CR object subresource status TotalIPCount should be changed or not
		isChange := false
		if ipPool.Status.TotalIPCount != nil {
			if *ipPool.Status.TotalIPCount != totalIPCount {
				logger.Sugar().Infof("IPPool '%s/%s' total IP count changed from '%d' to '%d'",
					ipPool.Namespace, ipPool.Name, *ipPool.Status.TotalIPCount, totalIPCount)
				isChange = true
			}
		} else {
			logger.Sugar().Infof("IPPool '%s/%s' status total IP count changed from nil to '%d'",
				ipPool.Namespace, ipPool.Name, totalIPCount)
			isChange = true
		}

		// check the IPPool CR object subresource status TotalIPCount and return,
		// because once subresource updated successfully the CR object resource version changed,
		// then we will meet conflict if we update other data.
		if isChange {
			ipPool.Status.TotalIPCount = &totalIPCount
			err = im.client.Status().Update(ctx, ipPool)
			if nil != err {
				logger.Sugar().Errorf("failed to update IPPool '%s/%s' status IP count to '%d', error: %v",
					ipPool.Namespace, ipPool.Name, totalIPCount, err)
				//TODO (Icarus9913): we will meet error here, should we return it and requeue ?
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, nil
		}
	} else {
		// remove finalizer for IPPool CR object when it's dying and no more IPs are used
		if len(ipPool.Status.AllocatedIPs) == 0 {
			err = im.RemoveFinalizer(ctx, request.Name)
			if nil != err {
				//TODO (Icarus9913): we will meet error here, should we return it and requeue ?
				logger.Sugar().Errorf("failed to remove IPPool '%+v' finalizer '%s', error: %v", request.NamespacedName, constant.SpiderFinalizer, err)
				return reconcile.Result{}, nil
			}
			logger.Sugar().Infof("remove IPPool '%+v' finalizer '%s' successfully", request.NamespacedName, constant.SpiderFinalizer)
		}
	}

	return reconcile.Result{}, nil
}
