// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package statefulsetmanager

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type StatefulSetManager interface {
	GetStatefulSetByName(ctx context.Context, namespace, name string, cached bool) (*appsv1.StatefulSet, error)
	ListStatefulSets(ctx context.Context, cached bool, opts ...client.ListOption) (*appsv1.StatefulSetList, error)
	IsValidStatefulSetPod(ctx context.Context, namespace, podName, podControllerType string) (bool, error)
}

type statefulSetManager struct {
	client    client.Client
	apiReader client.Reader
}

func NewStatefulSetManager(client client.Client, apiReader client.Reader) (StatefulSetManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	return &statefulSetManager{
		client:    client,
		apiReader: apiReader,
	}, nil
}

func (sm *statefulSetManager) GetStatefulSetByName(ctx context.Context, namespace, name string, cached bool) (*appsv1.StatefulSet, error) {
	reader := sm.apiReader
	if cached == constant.UseCache {
		reader = sm.client
	}

	var sts appsv1.StatefulSet
	if err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &sts); err != nil {
		return nil, err
	}

	return &sts, nil
}

func (sm *statefulSetManager) ListStatefulSets(ctx context.Context, cached bool, opts ...client.ListOption) (*appsv1.StatefulSetList, error) {
	reader := sm.apiReader
	if cached == constant.UseCache {
		reader = sm.client
	}

	var stsList appsv1.StatefulSetList
	if err := reader.List(ctx, &stsList, opts...); err != nil {
		return nil, err
	}

	return &stsList, nil
}

// IsValidStatefulSetPod only serves for StatefulSet pod, it will check the pod whether need to be cleaned up with the given params podNS, podName.
// Once the pod's controller StatefulSet was deleted, the pod's corresponding IPPool IP and Endpoint need to be cleaned up.
// Or the pod's controller StatefulSet decreased its replicas and the pod's index is out of replicas, it needs to be cleaned up too.
func (sm *statefulSetManager) IsValidStatefulSetPod(ctx context.Context, namespace, podName, podControllerType string) (bool, error) {
	if podControllerType != constant.KindStatefulSet {
		return false, fmt.Errorf("pod '%s/%s' is controlled by '%s' instead of StatefulSet", namespace, podName, podControllerType)
	}

	stsName, replicas, found := getStatefulSetNameAndOrdinal(podName)
	if !found {
		return false, nil
	}

	sts, err := sm.GetStatefulSetByName(ctx, namespace, stsName, constant.IgnoreCache)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}

	// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#start-ordinal
	if sts.Spec.Ordinals != nil {
		startIndex := int(sts.Spec.Ordinals.Start)
		endIndex := startIndex + int(*sts.Spec.Replicas) - 1
		if startIndex <= replicas && replicas <= endIndex {
			return true, nil
		}
		return false, nil
	}

	// The Pod controlled by StatefulSet is created or re-created.
	if replicas <= int(*sts.Spec.Replicas)-1 {
		return true, nil
	}

	// StatefulSet scaled down.
	return false, nil
}
