// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
)

var _ = Describe("ReservedIPManager", Label("reservedip_manager_test"), func() {
	Describe("New ReservedIPManager", func() {
		It("inputs nil client", func() {
			manager, err := reservedipmanager.NewReservedIPManager(nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test ReservedIPManager's method", func() {
		var count uint64
		var rIPName string
		var labels map[string]string
		var rIPT *spiderpoolv1.SpiderReservedIP

		BeforeEach(func() {
			atomic.AddUint64(&count, 1)
			rIPName = fmt.Sprintf("reservedip-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			rIPT = &spiderpoolv1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   rIPName,
					Labels: labels,
				},
				Spec: spiderpoolv1.ReservedIPSpec{},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			zero := int64(0)
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: &zero,
				PropagationPolicy:  &policy,
			}

			ctx := context.TODO()
			err := fakeClient.Delete(ctx, rIPT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetReservedIPByName", func() {
			It("gets non-existent ReservedIP", func() {
				ctx := context.TODO()
				rIP, err := rIPManager.GetReservedIPByName(ctx, rIPName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(rIP).To(BeNil())
			})

			It("gets an existing ReservedIP", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIP, err := rIPManager.GetReservedIPByName(ctx, rIPName)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIP).NotTo(BeNil())

				Expect(rIP).To(Equal(rIPT))
			})
		})

		Describe("ListReservedIPs", func() {
			It("failed to list ReservedIPs due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(rIPList).To(BeNil())
			})

			It("lists all ReservedIPs", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})

			It("filters results by label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})

			It("filters results by field selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, client.MatchingFields{metav1.ObjectNameField: rIPName})
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})
		})
	})
})
