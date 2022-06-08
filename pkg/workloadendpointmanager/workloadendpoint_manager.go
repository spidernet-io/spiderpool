// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

type WorkloadEndpointManager interface {
	RetriveIPAllocation(ctx context.Context, namespace, podName, containerID, nic string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error)
	UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error
	ClearCurrentIPs(ctx context.Context, namspace, podName, containerID string) error
	Delete(ctx context.Context, we *spiderpoolv1.WorkloadEndpoint) error
}

type workloadEndpointManager struct {
	client            client.Client
	maxHistoryRecords int
}

func NewWorkloadEndpointManager(c client.Client, maxHistoryRecords int) (WorkloadEndpointManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}

	return &workloadEndpointManager{
		client:            c,
		maxHistoryRecords: maxHistoryRecords,
	}, nil
}

func (r *workloadEndpointManager) RetriveIPAllocation(ctx context.Context, namespace, podName, containerID, nic string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error) {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		return nil, false, err
	}

	if we.Status.Current.ContainerID == containerID {
		for _, d := range we.Status.Current.IPs {
			if d.NIC == nic {
				return we.Status.Current, true, nil
			}
		}
	} else {
		if includeHistory {
			for _, a := range we.Status.History {
				if a.ContainerID == containerID {
					for _, d := range a.IPs {
						if d.NIC == nic {
							return &a, false, nil
						}
					}
				}
			}
		}
	}

	return nil, false, nil
}

func (r *workloadEndpointManager) UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
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
		if we.Status.Current.ContainerID == allocation.ContainerID {
			we.Status.Current.IPs = append(we.Status.Current.IPs, allocation.IPs...)
			we.Status.History[0].IPs = append(we.Status.History[0].IPs, allocation.IPs...)
		} else {
			we.Status.Current = allocation
			we.Status.History = append([]spiderpoolv1.PodIPAllocation{*allocation}, we.Status.History...)
			if len(we.Status.History) > r.maxHistoryRecords {
				we.Status.History = we.Status.History[:r.maxHistoryRecords]
			}
		}

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
