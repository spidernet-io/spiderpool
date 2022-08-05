// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Nodemanager", Label("unittest", "nodemanager test"), func() {

	DescribeTable("Test node manager", func(isMatch bool, genPod func() *corev1.Node) {

		node := genPod()
		selectorv1 := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "v1",
			}}
		selectorv2 := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"node": "p1",
			}}

		go func() {
			defer GinkgoRecover()
			succ, err := nodeManager.MatchLabelSelector(ctx, node.Name, selectorv1)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(succ).To(Equal(isMatch))
		}()

		succ, err := nodeManager.MatchLabelSelector(ctx, node.Name, selectorv2)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(succ).To(Equal(isMatch))

	},
		Entry("It will success when passing with match label", true, func() *corev1.Node {
			// define a node with label
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node1",
					Namespace: "group",
					Labels:    map[string]string{"app": "v1", "node": "p1"},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "127.0.2.5/15",
				},
			}
			// Delete all the node in the client
			err = k8sClient.DeleteAllOf(ctx, node)
			Expect(err).ShouldNot(HaveOccurred())

			// save node to k8s cluster
			err = k8sClient.Create(ctx, node)
			Expect(err).ShouldNot(HaveOccurred())
			return node
		}),
		Entry("It will fail when passing with mismatch label", false, func() *corev1.Node {
			// define a node with label
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node2",
					Namespace: "group",
					Labels:    map[string]string{"app": "v2", "node": "p2"},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "127.0.2.5/15",
				},
			}
			// Delete all the node in the client
			err = k8sClient.DeleteAllOf(ctx, node)
			Expect(err).ShouldNot(HaveOccurred())

			// save node to k8s cluster
			err = k8sClient.Create(ctx, node)
			Expect(err).ShouldNot(HaveOccurred())

			return node

		}))

})
