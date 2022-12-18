// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package statefulsetmanager_test

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
)

var _ = Describe("StatefulSetManager", Label("sts_manager_test"), func() {
	Describe("New StatefulSetManager", func() {
		It("inputs nil client", func() {
			manager, err := statefulsetmanager.NewStatefulSetManager(nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test StatefulSetManager's method", func() {
		var count uint64
		var namespace string
		var stsName string
		var labels map[string]string
		var stsT *appsv1.StatefulSet

		BeforeEach(func() {
			atomic.AddUint64(&count, 1)
			namespace = "default"
			stsName = fmt.Sprintf("sts-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			stsT = &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: appsv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName,
					Namespace: namespace,
					Labels:    labels,
				},
				Spec: appsv1.StatefulSetSpec{},
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
			err := fakeClient.Delete(ctx, stsT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetStatefulSetByName", func() {
			It("gets non-existent StatefulSet", func() {
				ctx := context.TODO()
				sts, err := stsManager.GetStatefulSetByName(ctx, namespace, stsName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(sts).To(BeNil())
			})

			It("gets an existing StatefulSet", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				sts, err := stsManager.GetStatefulSetByName(ctx, namespace, stsName)
				Expect(err).NotTo(HaveOccurred())
				Expect(sts).NotTo(BeNil())

				Expect(sts).To(Equal(stsT))
			})
		})

		Describe("ListStatefulSets", func() {
			It("failed to list StatefulSets due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(stsList).To(BeNil())
			})

			It("lists all StatefulSets", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(stsList.Items).NotTo(BeEmpty())

				hasSts := false
				for _, sts := range stsList.Items {
					if sts.Name == stsName {
						hasSts = true
						break
					}
				}
				Expect(hasSts).To(BeTrue())
			})

			It("filters results by Namespace", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, client.InNamespace(namespace))
				Expect(err).NotTo(HaveOccurred())
				Expect(stsList.Items).NotTo(BeEmpty())

				hasSts := false
				for _, sts := range stsList.Items {
					if sts.Name == stsName {
						hasSts = true
						break
					}
				}
				Expect(hasSts).To(BeTrue())
			})

			It("filters results by label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(stsList.Items).NotTo(BeEmpty())

				hasSts := false
				for _, sts := range stsList.Items {
					if sts.Name == stsName {
						hasSts = true
						break
					}
				}
				Expect(hasSts).To(BeTrue())
			})

			It("filters results by field selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, client.MatchingFields{metav1.ObjectNameField: stsName})
				Expect(err).NotTo(HaveOccurred())
				Expect(stsList.Items).NotTo(BeEmpty())

				hasSts := false
				for _, sts := range stsList.Items {
					if sts.Name == stsName {
						hasSts = true
						break
					}
				}
				Expect(hasSts).To(BeTrue())
			})
		})

		Describe("IsValidStatefulSetPod", func() {
			var index int32
			var stsPodName string
			var nonStsPodName string

			BeforeEach(func() {
				index = 1
				stsPodName = fmt.Sprintf("%s-%d", stsName, index)
				nonStsPodName = "other"
			})

			It("is not a Pod controlled by StatefulSet", func() {
				ctx := context.TODO()
				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, nonStsPodName, constant.OwnerUnknown)
				Expect(err).To(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("is a Pod controlled by StatefulSet, but the Pod name is invalid", func() {
				ctx := context.TODO()
				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, nonStsPodName, constant.OwnerStatefulSet)
				Expect(err).To(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("failed to parse replica string to int due to some unknown errors", func() {
				patches := gomonkey.ApplyFuncReturn(strconv.ParseInt, int64(0), constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, stsPodName, constant.OwnerStatefulSet)
				Expect(err).To(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("is a valid Pod controlled by StatefulSet, but the StatefulSet no longer exists", func() {
				ctx := context.TODO()
				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, stsPodName, constant.OwnerStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("used to be a Pod controlled by StatefulSet, but the StatefulSet scaled down", func() {
				stsT.Spec.Replicas = &index

				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, stsPodName, constant.OwnerStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("is a valid Pod controlled by StatefulSet", func() {
				replicas := index + 1
				stsT.Spec.Replicas = &replicas

				ctx := context.TODO()
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, stsPodName, constant.OwnerStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeTrue())
			})
		})
	})
})
