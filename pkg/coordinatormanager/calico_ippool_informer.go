// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"

	calicov1 "github.com/tigera/operator/pkg/apis/crd.projectcalico.org/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

var calicoController controller.Controller

func NewCalicoIPPoolController(mgr ctrl.Manager, workqueue workqueue.TypedRateLimitingInterface[string]) (controller.Controller, error) {
	if mgr == nil {
		return nil, fmt.Errorf("controller-runtime manager %w", constant.ErrMissingRequiredParam)
	}

	r := &calicoIPPoolReconciler{
		client:                     mgr.GetClient(),
		spiderCoordinatorWorkqueue: workqueue,
	}

	var err error
	if calicoController == nil {
		// only new one controller, avoid duplicate controller
		// // controller with name %s already exists. Controller names must be unique to avoid multiple controllers reporting to the same metric
		calicoController, err = controller.New(constant.KindSpiderCoordinator, mgr, controller.Options{Reconciler: r, SkipNameValidation: ptr.To(true)})
		if err != nil {
			return nil, err
		}
	}

	if err := calicoController.Watch(
		source.Kind[*calicov1.IPPool](
			mgr.GetCache(),
			&calicov1.IPPool{},
			&handler.TypedEnqueueRequestForObject[*calicov1.IPPool]{},
		),
	); err != nil {
		return nil, err
	}

	return calicoController, nil
}

type calicoIPPoolReconciler struct {
	client                     client.Client
	spiderCoordinatorWorkqueue workqueue.TypedRateLimitingInterface[string]
}

func (r *calicoIPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	InformerLogger.Sugar().Debugf("Watched Calico IPPool %v Enqueued", req.Name)
	r.spiderCoordinatorWorkqueue.Add(req.Name)
	return ctrl.Result{}, nil
}
