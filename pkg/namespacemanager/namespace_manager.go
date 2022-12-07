// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type NamespaceManager interface {
	GetNamespaceByName(ctx context.Context, nsName string) (*corev1.Namespace, error)
	ListNamespaces(ctx context.Context, opts ...client.ListOption) (*corev1.NamespaceList, error)
	MatchLabelSelector(ctx context.Context, nsName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type namespaceManager struct {
	client client.Client
}

func NewNamespaceManager(client client.Client) (NamespaceManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}

	return &namespaceManager{
		client: client,
	}, nil
}

func (nm *namespaceManager) GetNamespaceByName(ctx context.Context, nsName string) (*corev1.Namespace, error) {
	var ns corev1.Namespace
	if err := nm.client.Get(ctx, apitypes.NamespacedName{Name: nsName}, &ns); err != nil {
		return nil, err
	}

	return &ns, nil
}

func (nm *namespaceManager) ListNamespaces(ctx context.Context, opts ...client.ListOption) (*corev1.NamespaceList, error) {
	var nsList corev1.NamespaceList
	if err := nm.client.List(ctx, &nsList, opts...); err != nil {
		return nil, err
	}

	return &nsList, nil
}

func (nm *namespaceManager) MatchLabelSelector(ctx context.Context, nsName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	nsList, err := nm.ListNamespaces(
		ctx,
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: nsName},
	)
	if err != nil {
		return false, err
	}

	if len(nsList.Items) == 0 {
		return false, nil
	}

	return true, nil
}
