// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

type WorkloadEndpointManager interface {
	GetEndpointByName(ctx context.Context, namespace, podName string, cached bool) (*spiderpoolv1.SpiderEndpoint, error)
	ListEndpoints(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv1.SpiderEndpointList, error)
	DeleteEndpoint(ctx context.Context, endpoint *spiderpoolv1.SpiderEndpoint) error
	RemoveFinalizer(ctx context.Context, namespace, podName string) error
	PatchIPAllocationResults(ctx context.Context, containerID string, results []*types.AllocationResult, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController) error
	ReallocateCurrentIPAllocation(ctx context.Context, containerID, uid, nodeName string, endpoint *spiderpoolv1.SpiderEndpoint) error
}

type workloadEndpointManager struct {
	config    EndpointManagerConfig
	client    client.Client
	apiReader client.Reader
}

func NewWorkloadEndpointManager(config EndpointManagerConfig, client client.Client, apiReader client.Reader) (WorkloadEndpointManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	return &workloadEndpointManager{
		config:    setDefaultsForEndpointManagerConfig(config),
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i <= em.config.MaxConflictRetries; i++ {
		endpoint, err := em.GetEndpointByName(ctx, namespace, podName, constant.IgnoreCache)
		if err != nil {
			return client.IgnoreNotFound(err)
		}

		if !controllerutil.ContainsFinalizer(endpoint, constant.SpiderFinalizer) {
			return nil
		}

		controllerutil.RemoveFinalizer(endpoint, constant.SpiderFinalizer)
		if err := em.client.Update(ctx, endpoint); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == em.config.MaxConflictRetries {
				return fmt.Errorf("%w (%d times), failed to remove finalizer %s from Endpoint %s/%s", constant.ErrRetriesExhausted, em.config.MaxConflictRetries, constant.SpiderFinalizer, namespace, podName)
			}
			time.Sleep(time.Duration(r.Intn(1<<(i+1))) * em.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

func (em *workloadEndpointManager) PatchIPAllocationResults(ctx context.Context, containerID string, results []*types.AllocationResult, endpoint *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController) error {
	if pod == nil {
		return fmt.Errorf("pod %w", constant.ErrMissingRequiredParam)
	}

	allocation := spiderpoolv1.PodIPAllocation{
		ContainerID: containerID,
		UID:         string(pod.UID),
		Node:        pod.Spec.NodeName,
		IPs:         convert.ConvertResultsToIPDetails(results),
	}

	if endpoint == nil {
		endpoint = &spiderpoolv1.SpiderEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
			Status: spiderpoolv1.WorkloadEndpointStatus{
				Current:             allocation,
				OwnerControllerType: podController.Kind,
				OwnerControllerName: podController.Name,
				CreationTime:        metav1.Time{Time: time.Now()},
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

	if endpoint.Status.Current.ContainerID == containerID &&
		endpoint.Status.Current.UID == string(pod.UID) {
		return nil
	}

	endpoint.Status.Current = allocation
	return em.client.Update(ctx, endpoint)
}

func (em *workloadEndpointManager) ReallocateCurrentIPAllocation(ctx context.Context, containerID, uid, nodeName string, endpoint *spiderpoolv1.SpiderEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint %w", constant.ErrMissingRequiredParam)
	}

	if endpoint.Status.Current.ContainerID == containerID &&
		endpoint.Status.Current.UID == uid {
		return nil
	}

	endpoint.Status.Current.ContainerID = containerID
	endpoint.Status.Current.UID = uid
	endpoint.Status.Current.Node = nodeName

	return em.client.Update(ctx, endpoint)
}
