// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NodeManager utils", Label("node_manager_utils_test"), func() {
	Describe("IsNodeReady", func() {
		var node *corev1.Node
		BeforeEach(func() {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master",
				},
				Spec: corev1.NodeSpec{},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeMemoryPressure,
							Status: corev1.ConditionFalse,
						},
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
		})

		It("Node is ready", func() {
			isNodeReady := IsNodeReady(node)
			Expect(isNodeReady).To(BeTrue())
		})

		It("Node is not ready", func() {
			node.Status.Conditions[1].Status = corev1.ConditionUnknown
			isNodeReady := IsNodeReady(node)
			Expect(isNodeReady).To(BeFalse())
		})
	})
})
