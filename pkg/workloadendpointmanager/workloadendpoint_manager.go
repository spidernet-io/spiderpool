// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

type WorkloadEndpointManager interface {
	RetriveIPAllocation(ctx context.Context, namespace, podName, containerID string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error)
	UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error
	ClearCurrentIPs(ctx context.Context, namspace, podName, containerID string) error
	Delete(ctx context.Context, we *spiderpoolv1.WorkloadEndpoint) error
}

type workloadEndpointManager struct {
	client client.Client
}

func NewWorkloadEndpointManager(c client.Client) WorkloadEndpointManager {
	return &workloadEndpointManager{
		client: c,
	}
}

func (r *workloadEndpointManager) RetriveIPAllocation(ctx context.Context, namespace, podName, containerID string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error) {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		return nil, false, err
	}

	if we.Status.Current.ContainerID == containerID {
		return we.Status.Current, true, nil
	}

	if includeHistory {
		for _, a := range we.Status.History {
			if a.ContainerID == containerID {
				return &a, false, nil
			}
		}
	}

	return nil, false, nil
}

func (r *workloadEndpointManager) UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		newWE := &spiderpoolv1.WorkloadEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
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
		we.Status.Current = allocation
		we.Status.History = append([]spiderpoolv1.PodIPAllocation{*allocation}, we.Status.History...)
		if err := r.client.Status().Update(ctx, &we); err != nil {
			return err
		}
	}

	return nil
}

func (r *workloadEndpointManager) ClearCurrentIPs(ctx context.Context, namspace, podName, containerID string) error {
	return nil
}

func (r *workloadEndpointManager) Delete(ctx context.Context, we *spiderpoolv1.WorkloadEndpoint) error {
	return nil
}
