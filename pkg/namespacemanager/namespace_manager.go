// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager

import (
	"context"
	"encoding/json"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type NamespaceManager interface {
	GetNSDefaultPools(ctx context.Context, nsName string) ([]string, []string, error)
	MatchLabelSelector(ctx context.Context, nsName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type namespaceManager struct {
	client     client.Client
	runtimeMgr ctrl.Manager
}

func NewNamespaceManager(c client.Client, mgr ctrl.Manager) (NamespaceManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}
	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Namespace{}, metav1.ObjectNameField, func(raw client.Object) []string {
		namespace := raw.(*corev1.Namespace)
		return []string{namespace.Name}
	}); err != nil {
		return nil, err
	}

	return &namespaceManager{
		client:     c,
		runtimeMgr: mgr,
	}, nil
}

func (r *namespaceManager) GetNSDefaultPools(ctx context.Context, nsName string) ([]string, []string, error) {
	var namespace corev1.Namespace
	if err := r.client.Get(ctx, apitypes.NamespacedName{Name: nsName}, &namespace); err != nil {
		return nil, nil, err
	}

	var nsDefaultV4Pool types.AnnoNSDefautlV4PoolValue
	var nsDefaultV6Pool types.AnnoNSDefautlV6PoolValue
	if v, ok := namespace.Annotations[constant.AnnoNSDefautlV4Pool]; ok {
		if err := json.Unmarshal([]byte(v), &nsDefaultV4Pool); err != nil {
			return nil, nil, err
		}
	}

	if v, ok := namespace.Annotations[constant.AnnoNSDefautlV6Pool]; ok {
		if err := json.Unmarshal([]byte(v), &nsDefaultV6Pool); err != nil {
			return nil, nil, err
		}
	}

	return nsDefaultV4Pool, nsDefaultV6Pool, nil
}

func (r *namespaceManager) MatchLabelSelector(ctx context.Context, nsName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	var namespaces corev1.NamespaceList
	err = r.client.List(
		ctx,
		&namespaces,
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: nsName},
	)
	if err != nil {
		return false, err
	}

	if len(namespaces.Items) == 0 {
		return false, nil
	}

	return true, nil
}
