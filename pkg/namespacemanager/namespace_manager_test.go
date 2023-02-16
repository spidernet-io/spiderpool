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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
)

var _ = Describe("NamespaceManager", Label("namespace_manager_test"), func() {
	Describe("New NamespaceManager", func() {
		It("inputs nil client", func() {
			manager, err := namespacemanager.NewNamespaceManager(nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test NamespaceManager's method", func() {
		var count uint64
		var nsName string
		var labels map[string]string
		var nsT *corev1.Namespace

		BeforeEach(func() {
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
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &policy,
			}

			ctx := context.TODO()
			err := fakeClient.Delete(ctx, nsT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetNamespaceByName", func() {
			It("gets non-existent Namespace", func() {
				ctx := context.TODO()
				ns, err := nsManager.GetNamespaceByName(ctx, nsName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(ns).To(BeNil())
			})

			It("gets an existing Namespace", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				ns, err := nsManager.GetNamespaceByName(ctx, nsName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ns).NotTo(BeNil())

				Expect(ns).To(Equal(nsT))
			})
		})

		Describe("ListNamespaces", func() {
			It("failed to list Namespaces due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(nsList).To(BeNil())
			})

			It("lists all Namespaces", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx)
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
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, client.MatchingLabels(labels))
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
				ctx := context.TODO()
				err := fakeClient.Create(ctx, nsT)
				Expect(err).NotTo(HaveOccurred())

				nsList, err := nsManager.ListNamespaces(ctx, client.MatchingFields{metav1.ObjectNameField: nsName})
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
