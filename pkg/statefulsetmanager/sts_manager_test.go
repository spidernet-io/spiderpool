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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
)

var _ = Describe("StatefulSetManager", Label("sts_manager_test"), func() {
	Describe("New StatefulSetManager", func() {
		It("inputs nil client", func() {
			manager, err := statefulsetmanager.NewStatefulSetManager(nil, fakeAPIReader)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := statefulsetmanager.NewStatefulSetManager(fakeClient, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test StatefulSetManager's method", func() {
		var ctx context.Context

		var count uint64
		var namespace string
		var stsName string
		var labels map[string]string
		var stsT *appsv1.StatefulSet

		BeforeEach(func() {
			ctx = context.TODO()

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
				GracePeriodSeconds: ptr.To(int64(0)),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, stsT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    appsv1.GroupName,
					Version:  appsv1.SchemeGroupVersion.Version,
					Resource: "statefulsets",
				},
				stsT.Namespace,
				stsT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetStatefulSetByName", func() {
			It("gets non-existent StatefulSet", func() {
				sts, err := stsManager.GetStatefulSetByName(ctx, namespace, stsName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(sts).To(BeNil())
			})

			It("gets an existing StatefulSet through cache", func() {
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				sts, err := stsManager.GetStatefulSetByName(ctx, namespace, stsName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(sts).NotTo(BeNil())
				Expect(sts).To(Equal(stsT))
			})

			It("gets an existing StatefulSet through API Server", func() {
				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				sts, err := stsManager.GetStatefulSetByName(ctx, namespace, stsName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(sts).NotTo(BeNil())
				Expect(sts).To(Equal(stsT))
			})
		})

		Describe("ListStatefulSets", func() {
			It("failed to list StatefulSets due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(stsList).To(BeNil())
			})

			It("lists all StatefulSets through cache", func() {
				err := fakeClient.Create(ctx, stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, constant.UseCache)
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

			It("lists all StatefulSets through API Server", func() {
				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, constant.IgnoreCache)
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
				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, constant.IgnoreCache, client.InNamespace(namespace))
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
				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
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
				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				stsList, err := stsManager.ListStatefulSets(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: stsName})
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
			It("is not a Pod of StatefulSet", func() {
				valid, err := stsManager.IsValidStatefulSetPod(ctx, namespace, "orphan-pod", constant.KindPod)
				Expect(err).To(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("invalid StatefulSet pod with bad pod name replica parsing", func() {
				patches := gomonkey.ApplyFuncReturn(strconv.ParseInt, int64(0), constant.ErrUnknown)
				defer patches.Reset()

				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, 0), constant.KindStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("is a valid Pod controlled by StatefulSet, but the StatefulSet no longer exists", func() {
				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, 0), constant.KindStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("failed to get StatefulSets due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "Get", constant.ErrUnknown)
				defer patches.Reset()

				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, 0), constant.KindStatefulSet)
				Expect(err).To(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("used to be a Pod controlled by StatefulSet, but the StatefulSet scaled down", func() {
				replicas := int32(1)
				stsT.Spec.Replicas = &replicas

				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, replicas), constant.KindStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("is a valid Pod controlled by StatefulSet", func() {
				replicas := int32(1)
				stsT.Spec.Replicas = &replicas

				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, replicas-1), constant.KindStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeTrue())
			})

			It("invalid pod mismatches the StatefulSet start ordinal", func() {
				stsT.Spec.Replicas = ptr.To(int32(1))
				stsT.Spec.Ordinals = &appsv1.StatefulSetOrdinals{Start: 2}

				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, 0), constant.KindStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeFalse())
			})

			It("a valid pod matches the StatefulSet start ordinal", func() {
				stsT.Spec.Replicas = ptr.To(int32(1))
				stsT.Spec.Ordinals = &appsv1.StatefulSetOrdinals{Start: 2}

				err := tracker.Add(stsT)
				Expect(err).NotTo(HaveOccurred())

				valid, err := stsManager.IsValidStatefulSetPod(ctx, stsT.Namespace, fmt.Sprintf("%s-%d", stsName, 2), constant.KindStatefulSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(valid).To(BeTrue())
			})
		})
	})
})
