// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"
	"reflect"

	networkingv1 "k8s.io/api/networking/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

func NewServiceCIDRController(mgr ctrl.Manager, coordinatorName string) (controller.Controller, error) {
	if mgr == nil {
		return nil, fmt.Errorf("controller-runtime manager %w", constant.ErrMissingRequiredParam)
	}
	if len(coordinatorName) == 0 {
		return nil, fmt.Errorf("cluster coordinator name %w", constant.ErrMissingRequiredParam)
	}

	r := &serviceCIDRReconciler{
		client:          mgr.GetClient(),
		coordinatorName: coordinatorName,
	}

	c, err := controller.NewUnmanaged(constant.KindSpiderCoordinator, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}
	if err := c.Watch(source.Kind(mgr.GetCache(), &networkingv1.ServiceCIDR{}), &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	return c, nil
}

type serviceCIDRReconciler struct {
	client          client.Client
	coordinatorName string
}

func (r *serviceCIDRReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ipPoolList networkingv1.ServiceCIDRList
	if err := r.client.List(ctx, &ipPoolList); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	serviceCIDR := make([]string, 0, len(ipPoolList.Items))
	for _, p := range ipPoolList.Items {
		if p.DeletionTimestamp == nil {
			serviceCIDR = append(serviceCIDR, p.Spec.CIDRs...)
		}
	}

	var coordinator spiderpoolv2beta1.SpiderCoordinator
	if err := r.client.Get(ctx, types.NamespacedName{Name: r.coordinatorName}, &coordinator); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if coordinator.Status.Phase == Synced && reflect.DeepEqual(coordinator.Status.ServiceCIDR, serviceCIDR) {
		return ctrl.Result{}, nil
	}

	origin := coordinator.DeepCopy()
	coordinator.Status.Phase = Synced
	coordinator.Status.ServiceCIDR = serviceCIDR
	if err := r.client.Status().Patch(ctx, &coordinator, client.MergeFrom(origin)); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}
