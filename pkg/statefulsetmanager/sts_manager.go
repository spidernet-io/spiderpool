// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package statefulsetmanager

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type StatefulSetManager interface {
	GetStatefulSetByName(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error)
	ListStatefulSets(ctx context.Context, opts ...client.ListOption) (*appsv1.StatefulSetList, error)
	IsValidStatefulSetPod(ctx context.Context, podNS, podName, podControllerType string) (bool, error)
}

type statefulSetMgr struct {
	client     client.Client
	runtimeMgr ctrl.Manager
}

func NewStatefulSetManager(mgr ctrl.Manager) (StatefulSetManager, error) {
	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}

	stsMgr := statefulSetMgr{
		client:     mgr.GetClient(),
		runtimeMgr: mgr,
	}

	return &stsMgr, nil
}

func (sm *statefulSetMgr) GetStatefulSetByName(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet

	err := sm.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &sts)
	if nil != err {
		return nil, err
	}

	return &sts, nil
}

func (sm *statefulSetMgr) ListStatefulSets(ctx context.Context, opts ...client.ListOption) (*appsv1.StatefulSetList, error) {
	var stsList appsv1.StatefulSetList
	err := sm.client.List(ctx, &stsList, opts...)
	if nil != err {
		return nil, err
	}

	return &stsList, nil
}

// IsValidStatefulSetPod only serves for StatefulSet pod, it will check the pod whether need to be cleaned up with the given params podNS, podName.
// Once the pod's controller StatefulSet was deleted, the pod's corresponding IPPool IP and Endpoint need to be cleaned up.
// Or the pod's controller StatefulSet decreased its replicas and the pod's index is out of replicas, it needs to be cleaned up too.
func (sm *statefulSetMgr) IsValidStatefulSetPod(ctx context.Context, podNS, podName, podControllerType string) (bool, error) {
	if podControllerType != constant.OwnerStatefulSet {
		return false, fmt.Errorf("pod '%s/%s' owner controller type is '%s', not match StatefulSet type", podNS, podName, podControllerType)
	}

	statefulSetName, replicasIndex, found := getStatefulSetNameAndOrdinal(podName)
	if !found {
		return false, fmt.Errorf("failed to get pod '%s/%s' controller StatefulSet name and pod replicas index", podNS, podName)
	}

	isValid := true

	statefulSet, err := sm.GetStatefulSetByName(ctx, podNS, statefulSetName)
	if nil != err {
		if !apierrors.IsNotFound(err) {
			return false, err
		}

		// StatefulSet was deleted, just clean up IP and Endpoint
		isValid = false
	} else {
		switch {
		// pod restart
		case replicasIndex == int(*statefulSet.Spec.Replicas)-1:

		// StatefulSet decreased its replicas
		case replicasIndex > int(*statefulSet.Spec.Replicas)-1:
			isValid = false

		default:
		}
	}

	return isValid, nil
}
