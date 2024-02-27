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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
)

var _ = Describe("NodeManager", Label("node_manager_test"), func() {
	Describe("New NodeManager", func() {
		It("inputs nil client", func() {
			manager, err := nodemanager.NewNodeManager(nil, fakeAPIReader)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := nodemanager.NewNodeManager(fakeClient, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test NodeManager's method", func() {
		var ctx context.Context

		var count uint64
		var nodeName string
		var labels map[string]string
		var nodeT *corev1.Node

		BeforeEach(func() {
			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			nodeName = fmt.Sprintf("node-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			nodeT = &corev1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: corev1.SchemeGroupVersion.String(),
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
				GracePeriodSeconds: ptr.To(int64(0)),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, nodeT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    corev1.GroupName,
					Version:  corev1.SchemeGroupVersion.Version,
					Resource: "nodes",
				},
				nodeT.Namespace,
				nodeT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetNodeByName", func() {
			It("gets non-existent Node", func() {
				node, err := nodeManager.GetNodeByName(ctx, nodeName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(node).To(BeNil())
			})

			It("gets an existing Node through cache", func() {
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				node, err := nodeManager.GetNodeByName(ctx, nodeName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(node).NotTo(BeNil())
				Expect(node).To(Equal(nodeT))
			})

			It("gets an existing Node through API Server", func() {
				err := tracker.Add(nodeT)
				Expect(err).NotTo(HaveOccurred())

				node, err := nodeManager.GetNodeByName(ctx, nodeName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(node).NotTo(BeNil())
				Expect(node).To(Equal(nodeT))
			})
		})

		Describe("ListNodes", func() {
			It("failed to list Nodes due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(nodeList).To(BeNil())
			})

			It("lists all Nodes through cache", func() {
				err := fakeClient.Create(ctx, nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, constant.UseCache)
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

			It("lists all Nodes through API Server", func() {
				err := tracker.Add(nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, constant.IgnoreCache)
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
				err := tracker.Add(nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
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
				err := tracker.Add(nodeT)
				Expect(err).NotTo(HaveOccurred())

				nodeList, err := nodeManager.ListNodes(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: nodeName})
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
	})
})
