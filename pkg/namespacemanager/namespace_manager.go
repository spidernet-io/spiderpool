// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type NamespaceManager interface {
	GetNamespaceByName(ctx context.Context, nsName string, cached bool) (*corev1.Namespace, error)
	ListNamespaces(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.NamespaceList, error)
}

type namespaceManager struct {
	client    client.Client
	apiReader client.Reader
}

func NewNamespaceManager(client client.Client, apiReader client.Reader) (NamespaceManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	return &namespaceManager{
		client:    client,
		apiReader: apiReader,
	}, nil
}

func (nm *namespaceManager) GetNamespaceByName(ctx context.Context, nsName string, cached bool) (*corev1.Namespace, error) {
	reader := nm.apiReader
	if cached == constant.UseCache {
		reader = nm.client
	}

	var ns corev1.Namespace
	if err := reader.Get(ctx, apitypes.NamespacedName{Name: nsName}, &ns); err != nil {
		return nil, err
	}

	return &ns, nil
}

func (nm *namespaceManager) ListNamespaces(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.NamespaceList, error) {
	reader := nm.apiReader
	if cached == constant.UseCache {
		reader = nm.client
	}

	var nsList corev1.NamespaceList
	if err := reader.List(ctx, &nsList, opts...); err != nil {
		return nil, err
	}

	return &nsList, nil
}
