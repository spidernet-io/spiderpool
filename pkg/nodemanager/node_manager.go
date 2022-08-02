// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeManager interface {
	MatchLabelSelector(ctx context.Context, nodeName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type nodeManager struct {
	client     client.Client
	runtimeMgr ctrl.Manager
}

func NewNodeManager(mgr ctrl.Manager) (NodeManager, error) {
	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}
	return &nodeManager{
		client:     mgr.GetClient(),
		runtimeMgr: mgr,
	}, nil
}

// MatchLabelSelector will check whether the node matches the labelSelector or not
func (r *nodeManager) MatchLabelSelector(ctx context.Context, nodeName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}
	// Get the matches' node from client
	var nodes corev1.NodeList
	err = r.client.List(
		ctx,
		&nodes,
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: nodeName},
	)
	if err != nil {
		return false, err
	}

	if len(nodes.Items) == 0 {
		return false, nil
	}

	return true, nil
}
