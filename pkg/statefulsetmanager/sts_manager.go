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
	GetStatefulSetByName(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error)
	ListStatefulSets(ctx context.Context, opts ...client.ListOption) (*appsv1.StatefulSetList, error)
	IsValidStatefulSetPod(ctx context.Context, namespace, podName, podControllerType string) (bool, error)
}

type statefulSetManager struct {
	client client.Client
}

func NewStatefulSetManager(client client.Client) (StatefulSetManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s clinet %w", constant.ErrMissingRequiredParam)
	}

	return &statefulSetManager{
		client: client,
	}, nil
}

func (sm *statefulSetManager) GetStatefulSetByName(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	if err := sm.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &sts); err != nil {
		return nil, err
	}

	return &sts, nil
}

func (sm *statefulSetManager) ListStatefulSets(ctx context.Context, opts ...client.ListOption) (*appsv1.StatefulSetList, error) {
	var stsList appsv1.StatefulSetList
	if err := sm.client.List(ctx, &stsList, opts...); err != nil {
		return nil, err
	}

	return &stsList, nil
}

// IsValidStatefulSetPod only serves for StatefulSet pod, it will check the pod whether need to be cleaned up with the given params podNS, podName.
// Once the pod's controller StatefulSet was deleted, the pod's corresponding IPPool IP and Endpoint need to be cleaned up.
// Or the pod's controller StatefulSet decreased its replicas and the pod's index is out of replicas, it needs to be cleaned up too.
func (sm *statefulSetManager) IsValidStatefulSetPod(ctx context.Context, namespace, podName, podControllerType string) (bool, error) {
	if podControllerType != constant.OwnerStatefulSet {
		return false, fmt.Errorf("pod '%s/%s' is controlled by '%s' instead of StatefulSet", namespace, podName, podControllerType)
	}

	stsName, replicas, found := getStatefulSetNameAndOrdinal(podName)
	if !found {
		return false, fmt.Errorf("failed to parse the name and replica of its StatefulSet controller from the name of Pod '%s/%s'", namespace, podName)
	}

	sts, err := sm.GetStatefulSetByName(ctx, namespace, stsName)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}

	// Pod controlled by StatefulSet is created or recreated.
	if replicas <= int(*sts.Spec.Replicas)-1 {
		return true, nil
	}

	// StatefulSet scaled down.
	return false, nil
}
