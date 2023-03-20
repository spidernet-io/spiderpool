// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var _ = Describe("IPPoolManager", Label("ippool_manager_test"), func() {
	Describe("New IPPoolManager", func() {
		It("sets default config", func() {
			manager, err := ippoolmanager.NewIPPoolManager(
				ippoolmanager.IPPoolManagerConfig{},
				fakeClient,
				fakeAPIReader,
				mockRIPManager,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(manager).NotTo(BeNil())
		})

		It("inputs nil client", func() {
			manager, err := ippoolmanager.NewIPPoolManager(
				ippoolmanager.IPPoolManagerConfig{},
				nil,
				fakeAPIReader,
				mockRIPManager,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := ippoolmanager.NewIPPoolManager(
				ippoolmanager.IPPoolManagerConfig{},
				fakeClient,
				nil,
				mockRIPManager,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil reserved-IP manager", func() {
			manager, err := ippoolmanager.NewIPPoolManager(
				ippoolmanager.IPPoolManagerConfig{},
				fakeClient,
				fakeAPIReader,
				nil,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test IPPoolManager's method", func() {
		var ctx context.Context

		var count uint64
		var ipPoolName string
		var labels map[string]string
		var ipPoolT *spiderpoolv2beta1.SpiderIPPool

		BeforeEach(func() {
			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			ipPoolName = fmt.Sprintf("ippool-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			ipPoolT = &spiderpoolv2beta1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderIPPool,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   ipPoolName,
					Labels: labels,
				},
				Spec: spiderpoolv2beta1.IPPoolSpec{},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, ipPoolT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderippools",
				},
				ipPoolT.Namespace,
				ipPoolT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetIPPoolByName", func() {
			It("gets non-existent IPPool", func() {
				ipPool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(ipPool).To(BeNil())
			})

			It("gets an existing IPPool through cache", func() {
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPool).NotTo(BeNil())
				Expect(ipPool).To(Equal(ipPoolT))
			})

			It("gets an existing IPPool through API Server", func() {
				err := tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPool).NotTo(BeNil())
				Expect(ipPool).To(Equal(ipPoolT))
			})
		})

		Describe("ListIPPools", func() {
			It("failed to list IPPools due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(ipPoolList).To(BeNil())
			})

			It("lists all IPPools through cache", func() {
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})

			It("lists all IPPools through API Server", func() {
				err := tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})

			It("filters results by label selector", func() {
				err := tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})

			It("filters results by field selector", func() {
				err := tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: ipPoolName})
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})
		})
	})
})
