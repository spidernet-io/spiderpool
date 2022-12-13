// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
)

var _ = Describe("NodeManager", Label("node_manager_test"), func() {
	Describe("New NodeManager", func() {
		It("inputs nil client", func() {
			manager, err := nodemanager.NewNodeManager(nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test NodeManager's method", func() {
		var count uint64
		var nodeName string
		var labels map[string]string
		var nodeT *corev1.Node

		BeforeEach(func() {
			atomic.AddUint64(&count, 1)
			nodeName = fmt.Sprintf("node-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			nodeT = &corev1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   nodeName,
					Labels: labels,
				},
				Spec: corev1.NodeSpec{},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &policy,
			}

			ctx := context.TODO()
			err := fakeClient.Delete(ctx, nodeT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetNodeByName", func() {
			It("gets non-existent Node", func() {
				ctx := context.TODO()
				node, err := nodeManager.GetNodeByName(ctx, nodeName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(node).To(BeNil())
			})

			It("gets an existing Node", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				node, err := nodeManager.GetNodeByName(ctx, nodeName)
				Expect(err).NotTo(HaveOccurred())
				Expect(node).NotTo(BeNil())

				Expect(node).To(Equal(nodeT))
			})
		})

		Describe("ListNodes", func() {
			It("failed to list Nodes due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(nodeList).To(BeNil())
			})

			It("lists all Nodes", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(nodeList.Items).NotTo(BeEmpty())

				hasNode := false
				for _, node := range nodeList.Items {
					if node.Name == nodeName {
						hasNode = true
						break
					}
				}
				Expect(hasNode).To(BeTrue())
			})

			It("filters results by label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(nodeList.Items).NotTo(BeEmpty())

				hasNode := false
				for _, node := range nodeList.Items {
					if node.Name == nodeName {
						hasNode = true
						break
					}
				}
				Expect(hasNode).To(BeTrue())
			})

			It("filters results by field selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, client.MatchingFields{metav1.ObjectNameField: nodeName})
				Expect(err).NotTo(HaveOccurred())
				Expect(nodeList.Items).NotTo(BeEmpty())

				hasNode := false
				for _, node := range nodeList.Items {
					if node.Name == nodeName {
						hasNode = true
						break
					}
				}
				Expect(hasNode).To(BeTrue())
			})
		})

		Describe("MatchLabelSelector", func() {
			It("checks non-existent Node", func() {
				ctx := context.TODO()
				match, err := nodeManager.MatchLabelSelector(ctx, nodeName, &metav1.LabelSelector{MatchLabels: labels})
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(BeFalse())
			})

			It("matches invalid label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				invalidSelector := &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"": "",
					},
				}
				match, err := nodeManager.MatchLabelSelector(ctx, nodeName, invalidSelector)
				Expect(err).To(HaveOccurred())
				Expect(match).To(BeFalse())
			})

			It("failed to list Nodes due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				match, err := nodeManager.MatchLabelSelector(ctx, nodeName, &metav1.LabelSelector{MatchLabels: labels})
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(match).To(BeFalse())
			})

			It("matches nothing", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				match, err := nodeManager.MatchLabelSelector(ctx, nodeName, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(BeFalse())
			})

			It("matches the label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				match, err := nodeManager.MatchLabelSelector(ctx, nodeName, &metav1.LabelSelector{MatchLabels: labels})
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(BeTrue())
			})
		})
	})
})
