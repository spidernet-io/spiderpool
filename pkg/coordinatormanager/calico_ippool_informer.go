// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"
	"reflect"

	calicov1 "github.com/tigera/operator/pkg/apis/crd.projectcalico.org/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

func NewCalicoIPPoolController(mgr ctrl.Manager, coordinatorName string) (controller.Controller, error) {
	if mgr == nil {
		return nil, fmt.Errorf("controller-runtime manager %w", constant.ErrMissingRequiredParam)
	}
	if len(coordinatorName) == 0 {
		return nil, fmt.Errorf("cluster coordinator name %w", constant.ErrMissingRequiredParam)
	}

	r := &calicoIPPoolReconciler{
		client:          mgr.GetClient(),
		coordinatorName: coordinatorName,
	}

	c, err := controller.NewUnmanaged(constant.KindSpiderCoordinator, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &calicov1.IPPool{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	return c, nil
}

type calicoIPPoolReconciler struct {
	client          client.Client
	coordinatorName string
}

func (r *calicoIPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ipPoolList calicov1.IPPoolList
	if err := r.client.List(ctx, &ipPoolList); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	podCIDR := make([]string, 0, len(ipPoolList.Items))
	for _, p := range ipPoolList.Items {
		if p.DeletionTimestamp == nil && !p.Spec.Disabled {
			podCIDR = append(podCIDR, p.Spec.CIDR)
		}
	}

	var coordinator spiderpoolv2beta1.SpiderCoordinator
	if err := r.client.Get(ctx, types.NamespacedName{Name: r.coordinatorName}, &coordinator); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if coordinator.Status.Phase == synced && reflect.DeepEqual(coordinator.Status.PodCIDR, podCIDR) {
		return ctrl.Result{}, nil
	}

	origin := coordinator.DeepCopy()
	coordinator.Status.Phase = synced
	coordinator.Status.PodCIDR = podCIDR
	if err := r.client.Status().Patch(ctx, &coordinator, client.MergeFrom(origin)); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}
