// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"

	calicov1 "github.com/tigera/operator/pkg/apis/crd.projectcalico.org/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

func NewCalicoIPPoolController(mgr ctrl.Manager, workQueue workqueue.RateLimitingInterface) (controller.Controller, error) {
	if mgr == nil {
		return nil, fmt.Errorf("controller-runtime manager %w", constant.ErrMissingRequiredParam)
	}

	r := &calicoIPPoolReconciler{
		client:                     mgr.GetClient(),
		spiderCoordinatorWorkqueue: workQueue,
	}

	c, err := controller.NewUnmanaged(constant.KindSpiderCoordinator, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &calicov1.IPPool{}), &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	return c, nil
}

type calicoIPPoolReconciler struct {
	client                     client.Client
	spiderCoordinatorWorkqueue workqueue.RateLimitingInterface
}

func (r *calicoIPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	InformerLogger.Sugar().Debugf("Watched Calico IPPool %v Enqueued", req.Name)
	r.spiderCoordinatorWorkqueue.Add(fmt.Sprintf("CalicoIPPool/%v", req.Name))
	return ctrl.Result{}, nil
}
