// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type NodeManager interface {
	GetNodeByName(ctx context.Context, nodeName string) (*corev1.Node, error)
	ListNodes(ctx context.Context, opts ...client.ListOption) (*corev1.NodeList, error)
}

type nodeManager struct {
	client client.Client
}

func NewNodeManager(client client.Client) (NodeManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}

	return &nodeManager{
		client: client,
	}, nil
}

func (nm *nodeManager) GetNodeByName(ctx context.Context, nodeName string) (*corev1.Node, error) {
	var node corev1.Node
	if err := nm.client.Get(ctx, apitypes.NamespacedName{Name: nodeName}, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

func (nm *nodeManager) ListNodes(ctx context.Context, opts ...client.ListOption) (*corev1.NodeList, error) {
	var nodeList corev1.NodeList
	if err := nm.client.List(ctx, &nodeList, opts...); err != nil {
		return nil, err
	}

	return &nodeList, nil
}
