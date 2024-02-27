// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager_test

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
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
)

var _ = Describe("NamespaceManager", Label("namespace_manager_test"), func() {
	Describe("New NamespaceManager", func() {
		It("inputs nil client", func() {
			manager, err := namespacemanager.NewNamespaceManager(nil, fakeAPIReader)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := namespacemanager.NewNamespaceManager(fakeClient, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test NamespaceManager's method", func() {
		var ctx context.Context

		var count uint64
		var nsName string
		var labels map[string]string
		var nsT *corev1.Namespace

		BeforeEach(func() {
			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			nsName = fmt.Sprintf("namespace-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			nsT = &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   nsName,
					Labels: labels,
				},
				Spec: corev1.NamespaceSpec{},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: ptr.To(int64(0)),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, nsT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    corev1.GroupName,
					Version:  corev1.SchemeGroupVersion.Version,
					Resource: "namespaces",
				},
				nsT.Namespace,
				nsT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetNamespaceByName", func() {
			It("gets non-existent Namespace", func() {
				ns, err := nsManager.GetNamespaceByName(ctx, nsName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(ns).To(BeNil())
			})

			It("gets an existing Namespace through cache", func() {
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				ns, err := nsManager.GetNamespaceByName(ctx, nsName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(ns).NotTo(BeNil())
				Expect(ns).To(Equal(nsT))
			})

			It("gets an existing Namespace through API Server", func() {
				err := tracker.Add(nsT)
				Expect(err).NotTo(HaveOccurred())

				ns, err := nsManager.GetNamespaceByName(ctx, nsName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(ns).NotTo(BeNil())
				Expect(ns).To(Equal(nsT))
			})
		})

		Describe("ListNamespaces", func() {
			It("failed to list Namespaces due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(nsList).To(BeNil())
			})

			It("lists all Namespaces through cache", func() {
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(nsList.Items).NotTo(BeEmpty())

				hasNS := false
				for _, ns := range nsList.Items {
					if ns.Name == nsName {
						hasNS = true
						break
					}
				}
				Expect(hasNS).To(BeTrue())
			})

			It("lists all Namespaces through API Server", func() {
				err := tracker.Add(nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(nsList.Items).NotTo(BeEmpty())

				hasNS := false
				for _, ns := range nsList.Items {
					if ns.Name == nsName {
						hasNS = true
						break
					}
				}
				Expect(hasNS).To(BeTrue())
			})

			It("filters results by label selector", func() {
				err := tracker.Add(nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(nsList.Items).NotTo(BeEmpty())

				hasNS := false
				for _, ns := range nsList.Items {
					if ns.Name == nsName {
						hasNS = true
						break
					}
				}
				Expect(hasNS).To(BeTrue())
			})

			It("filters results by field selector", func() {
				err := tracker.Add(nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: nsName})
				Expect(err).NotTo(HaveOccurred())
				Expect(nsList.Items).NotTo(BeEmpty())

				hasNS := false
				for _, ns := range nsList.Items {
					if ns.Name == nsName {
						hasNS = true
						break
					}
				}
				Expect(hasNS).To(BeTrue())
			})
		})
	})
})
