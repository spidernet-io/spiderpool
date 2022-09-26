// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeManager interface {
	GetNodeByName(ctx context.Context, nodeName string) (*corev1.Node, error)
	ListNodes(ctx context.Context, opts ...client.ListOption) (*corev1.NodeList, error)
	MatchLabelSelector(ctx context.Context, nodeName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type nodeManager struct {
	client client.Client
}

func NewNodeManager(client client.Client) (NodeManager, error) {
	if client == nil {
		return nil, errors.New("k8s client must be specified")
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

func (nm *nodeManager) MatchLabelSelector(ctx context.Context, nodeName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	nodeList, err := nm.ListNodes(
		ctx,
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: nodeName},
	)
	if err != nil {
		return false, err
	}

	if len(nodeList.Items) == 0 {
		return false, nil
	}

	return true, nil
}
