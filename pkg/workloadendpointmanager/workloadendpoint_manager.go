// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

type WorkloadEndpointManager interface {
	RetriveIPAllocation(ctx context.Context, namespace, podName, containerID, nic string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error)
	UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error
	ClearCurrentIPAllocation(ctx context.Context, namespace, podName, containerID, nic string) error
}

type workloadEndpointManager struct {
	client            client.Client
	runtimeMgr        ctrl.Manager
	maxHistoryRecords int
}

func NewWorkloadEndpointManager(c client.Client, mgr ctrl.Manager, maxHistoryRecords int) (WorkloadEndpointManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}
	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}

	return &workloadEndpointManager{
		client:            c,
		runtimeMgr:        mgr,
		maxHistoryRecords: maxHistoryRecords,
	}, nil
}

func (r *workloadEndpointManager) RetriveIPAllocation(ctx context.Context, namespace, podName, containerID, nic string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error) {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if we.Status.Current != nil && we.Status.Current.ContainerID == containerID {
		for _, d := range we.Status.Current.IPs {
			if d.NIC == nic {
				return we.Status.Current, true, nil
			}
		}
	}

	if includeHistory {
		for _, a := range we.Status.History[1:] {
			if a.ContainerID == containerID {
				for _, d := range a.IPs {
					if d.NIC == nic {
						return &a, false, nil
					}
				}
			}
		}
	}

	return nil, false, nil
}

func (r *workloadEndpointManager) UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error {
	logger := logutils.FromContext(ctx)

	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		var pod corev1.Pod
		if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
			return err
		}

		newWE := &spiderpoolv1.WorkloadEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
		}

		controllerutil.AddFinalizer(newWE, constant.SpiderWorkloadEndpointFinalizer)
		if err := controllerutil.SetOwnerReference(&pod, newWE, r.runtimeMgr.GetScheme()); err != nil {
			return nil
		}

		if err := r.client.Create(ctx, newWE); err != nil {
			return err
		}

		newWE.Status.Current = allocation
		newWE.Status.History = []spiderpoolv1.PodIPAllocation{*allocation}
		if err := r.client.Status().Update(ctx, newWE); err != nil {
			return err
		}
	} else {
		if we.Status.Current != nil && we.Status.Current.ContainerID == allocation.ContainerID {
			we.Status.Current.IPs = append(we.Status.Current.IPs, allocation.IPs...)
			we.Status.History[0].IPs = append(we.Status.History[0].IPs, allocation.IPs...)
		} else {
			we.Status.Current = allocation
			we.Status.History = append([]spiderpoolv1.PodIPAllocation{*allocation}, we.Status.History...)
			if len(we.Status.History) > r.maxHistoryRecords {
				logger.Sugar().Warnf("threshold of historical IP allocation records(<=%d) exceeded", r.maxHistoryRecords)
				we.Status.History = we.Status.History[:r.maxHistoryRecords]
			}
		}

		if err := r.client.Status().Update(ctx, &we); err != nil {
			return err
		}
	}

	return nil
}

func (r *workloadEndpointManager) ClearCurrentIPAllocation(ctx context.Context, namespace, podName, containerID, nic string) error {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if we.Status.Current == nil {
		return nil
	}

	if we.Status.Current.ContainerID != containerID {
		return nil
	}

	we.Status.Current = nil
	if err := r.client.Status().Update(ctx, &we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}
