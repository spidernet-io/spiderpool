// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager

import corev1 "k8s.io/api/core/v1"

func IsNodeReady(node *corev1.Node) bool {
	var readyCondition corev1.NodeCondition
	for _, tmpCondition := range node.Status.Conditions {
		if tmpCondition.Type == corev1.NodeReady {
			readyCondition = tmpCondition
			break
		}
	}

	return readyCondition.Status == corev1.ConditionTrue
}
