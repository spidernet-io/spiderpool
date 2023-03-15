// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/utils/retry"
)

type WorkloadEndpointManager interface {
	GetEndpointByName(ctx context.Context, namespace, podName string, cached bool) (*spiderpoolv1.SpiderEndpoint, error)
	ListEndpoints(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv1.SpiderEndpointList, error)
	DeleteEndpoint(ctx context.Context, endpoint *spiderpoolv1.SpiderEndpoint) error
	RemoveFinalizer(ctx context.Context, namespace, podName string) error
	PatchIPAllocationResults(ctx context.Context, results []*types.AllocationResult, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController) error
	ReallocateCurrentIPAllocation(ctx context.Context, uid, nodeName string, endpoint *spiderpoolv1.SpiderEndpoint) error
}

type workloadEndpointManager struct {
	client    client.Client
	apiReader client.Reader
}

func NewWorkloadEndpointManager(client client.Client, apiReader client.Reader) (WorkloadEndpointManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	return &workloadEndpointManager{
		client:    client,
		apiReader: apiReader,
	}, nil
}

func (em *workloadEndpointManager) GetEndpointByName(ctx context.Context, namespace, podName string, cached bool) (*spiderpoolv1.SpiderEndpoint, error) {
	reader := em.apiReader
	if cached == constant.UseCache {
		reader = em.client
	}

	var endpoint spiderpoolv1.SpiderEndpoint
	if err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &endpoint); nil != err {
		return nil, err
	}

	return &endpoint, nil
}

func (em *workloadEndpointManager) ListEndpoints(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv1.SpiderEndpointList, error) {
	reader := em.apiReader
	if cached == constant.UseCache {
		reader = em.client
	}

	var endpointList spiderpoolv1.SpiderEndpointList
	if err := reader.List(ctx, &endpointList, opts...); err != nil {
		return nil, err
	}

	return &endpointList, nil
}

func (em *workloadEndpointManager) DeleteEndpoint(ctx context.Context, endpoint *spiderpoolv1.SpiderEndpoint) error {
	if err := em.client.Delete(ctx, endpoint); err != nil {
		return client.IgnoreNotFound(err)
	}

	return nil
}

func (em *workloadEndpointManager) RemoveFinalizer(ctx context.Context, namespace, podName string) error {
	backoff := retry.DefaultRetry
	steps := backoff.Steps
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		endpoint, err := em.GetEndpointByName(ctx, namespace, podName, constant.IgnoreCache)
		if err != nil {
			return client.IgnoreNotFound(err)
		}

		if !controllerutil.ContainsFinalizer(endpoint, constant.SpiderFinalizer) {
			return nil
		}

		controllerutil.RemoveFinalizer(endpoint, constant.SpiderFinalizer)
		return em.client.Update(ctx, endpoint)
	})
	if err != nil {
		if err == wait.ErrWaitTimeout {
			err = fmt.Errorf("%w (%d times), failed to remove finalizer %s from Endpoint %s/%s", constant.ErrRetriesExhausted, steps, constant.SpiderFinalizer, namespace, podName)
		}
		return err
	}

	return nil
}

func (em *workloadEndpointManager) PatchIPAllocationResults(ctx context.Context, results []*types.AllocationResult, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController) error {
	if pod == nil {
		return fmt.Errorf("pod %w", constant.ErrMissingRequiredParam)
	}

	if endpoint == nil {
		endpoint = &spiderpoolv1.SpiderEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
			Status: spiderpoolv1.WorkloadEndpointStatus{
				Current: spiderpoolv1.PodIPAllocation{
					UID:  string(pod.UID),
					Node: pod.Spec.NodeName,
					IPs:  convert.ConvertResultsToIPDetails(results),
				},
				OwnerControllerType: podController.Kind,
				OwnerControllerName: podController.Name,
			},
		}

		// Do not set ownerReference for Endpoint when its corresponding Pod is
		// controlled by StatefulSet. Once the Pod of StatefulSet is recreated,
		// we can immediately retrieve the old IP allocation results from the
		// Endpoint without worrying about the cascading deletion of the Endpoint.
		if podController.Kind != constant.KindStatefulSet {
			if err := controllerutil.SetOwnerReference(pod, endpoint, em.client.Scheme()); err != nil {
				return err
			}
		}
		controllerutil.AddFinalizer(endpoint, constant.SpiderFinalizer)
		return em.client.Create(ctx, endpoint)
	}

	if endpoint.Status.Current.UID != string(pod.UID) {
		return nil
	}

	// TODO(iiiceoo): Only append records with different NIC.
	endpoint.Status.Current.IPs = append(endpoint.Status.Current.IPs, convert.ConvertResultsToIPDetails(results)...)
	return em.client.Update(ctx, endpoint)
}

func (em *workloadEndpointManager) ReallocateCurrentIPAllocation(ctx context.Context, uid, nodeName string, endpoint *spiderpoolv1.SpiderEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint %w", constant.ErrMissingRequiredParam)
	}

	if endpoint.Status.Current.UID == uid {
		return nil
	}

	endpoint.Status.Current.UID = uid
	endpoint.Status.Current.Node = nodeName

	return em.client.Update(ctx, endpoint)
}
