// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("IPPoolManager utils", Label("ippool_manager_utils_test"), func() {
	var count uint64
	var ipPoolName string
	var ipPoolT *spiderpoolv1.SpiderIPPool
	var labels map[string]string

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)
		ipPoolName = fmt.Sprintf("ippool-%v", count)
		labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
		ipPoolT = &spiderpoolv1.SpiderIPPool{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.SpiderIPPoolKind,
				APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   ipPoolName,
				Labels: labels,
			},
			Spec: spiderpoolv1.IPPoolSpec{},
		}
		ipPoolT.SetUID(uuid.NewUUID())
		ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
		ipPoolT.Spec.Vlan = pointer.Int64(0)
		ipPoolT.Spec.Subnet = "172.20.10.0/24"
		ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
			[]string{
				"172.20.10.2",
				"172.20.10.3-172.20.10.100",
			}...,
		)
	})

	var deleteOption *client.DeleteOptions

	AfterEach(func() {
		policy := metav1.DeletePropagationForeground
		deleteOption = &client.DeleteOptions{
			GracePeriodSeconds: pointer.Int64(0),
			PropagationPolicy:  &policy,
		}

		ctx := context.TODO()
		err := fakeClient.Delete(ctx, ipPoolT, deleteOption)
		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
	})

	Describe("ShouldScaleIPPool", func() {

		It("scale an IPPool with the pool's AutoDesiredIPCount is nil", func() {
			ipPoolT.Status.AutoDesiredIPCount = nil
			ctx := context.TODO()
			err := fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			isOk := ippoolmanager.ShouldScaleIPPool(ipPoolT)
			Expect(isOk).To(BeFalse())
		})

		When("scale an IPPool with the pool's AutoDesiredIPCount not nil", func() {
			It("assemble total IPs not equal to AutoDesiredIPCount", func() {
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(10)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				isOk := ippoolmanager.ShouldScaleIPPool(ipPoolT)
				Expect(isOk).To(BeTrue())
			})

			It("assemble total IPs equal to AutoDesiredIPCount", func() {
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(2)
				ipPoolT.Spec.IPs = []string{
					"172.20.10.2",
					"172.20.10.3",
				}
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				isOk := ippoolmanager.ShouldScaleIPPool(ipPoolT)
				Expect(isOk).To(BeFalse())
			})
		})
	})

	Describe("IsAutoCreatedIPPool", func() {
		It("label 'ipam.spidernet.io/owner-application' does not exist", func() {
			ctx := context.TODO()
			err := fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			isOk := ippoolmanager.IsAutoCreatedIPPool(ipPoolT)
			Expect(isOk).To(BeFalse())
		})

		It("label 'ipam.spidernet.io/owner-application' exist", func() {
			labels = map[string]string{"ipam.spidernet.io/owner-application": ipPoolName}
			ipPoolT.Labels = labels
			ctx := context.TODO()
			err := fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			isOk := ippoolmanager.IsAutoCreatedIPPool(ipPoolT)
			Expect(isOk).To(BeTrue())
		})
	})
})
