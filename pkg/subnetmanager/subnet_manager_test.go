// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager_test

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

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("SubnetManager_test", Label("subnet_manager_test"), func() {
	Describe("New SubnetManager", func() {
		It("sets default config", func() {
			manager, err := subnetmanager.NewSubnetManager(subnetmanager.SubnetManagerConfig{}, fakeClient, ipPoolManager, fakeClient.Scheme())
			Expect(err).NotTo(HaveOccurred())
			Expect(manager).NotTo(BeNil())
		})

		It("inputs nil client", func() {
			manager, err := subnetmanager.NewSubnetManager(subnetmanager.SubnetManagerConfig{}, nil, ipPoolManager, fakeClient.Scheme())
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil IPPool manager", func() {
			manager, err := subnetmanager.NewSubnetManager(subnetmanager.SubnetManagerConfig{}, fakeClient, nil, fakeClient.Scheme())
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil Scheme", func() {
			manager, err := subnetmanager.NewSubnetManager(subnetmanager.SubnetManagerConfig{}, fakeClient, ipPoolManager, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test SubnetManager's method", func() {
		var count uint64
		var subnetName string
		var subnetT *spiderpoolv1.SpiderSubnet
		var labels map[string]string

		BeforeEach(func() {
			atomic.AddUint64(&count, 1)
			subnetName = fmt.Sprintf("Subnet-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			subnetT = &spiderpoolv1.SpiderSubnet{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderSubnetKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   subnetName,
					Labels: labels,
				},
				Spec: spiderpoolv1.SubnetSpec{},
			}
			ipVersion := constant.IPv4
			subnet := "172.18.43.0/24"
			subnetT.Spec.IPVersion = &ipVersion
			subnetT.Spec.Subnet = subnet
		})
		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &policy,
			}

			ctx := context.TODO()
			err := fakeClient.Delete(ctx, subnetT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetSubnetByName", func() {
			It("gets non-existent Subnet", func() {
				ctx := context.TODO()
				subnet, err := subnetManager.GetSubnetByName(ctx, subnetName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(subnet).To(BeNil())
			})

			It("gets an existing Subnet", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnet, err := subnetManager.GetSubnetByName(ctx, subnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(subnet).NotTo(BeNil())
				Expect(subnet).To(Equal(subnetT))
			})
		})

		Describe("ListSubnets", func() {
			It("failed to list IPPools due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetList, err := subnetManager.ListSubnets(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(subnetList).To(BeNil())
			})

			It("lists all Subnets", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetList, err := subnetManager.ListSubnets(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(subnetList.Items).NotTo(BeEmpty())

				hasSubnet := false
				for _, subnet := range subnetList.Items {
					if subnet.Name == subnetName {
						hasSubnet = true
						break
					}
				}
				Expect(hasSubnet).To(BeTrue())
			})

			It("filters results by label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetList, err := subnetManager.ListSubnets(ctx, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(subnetList.Items).NotTo(BeEmpty())

				hasSubnet := false
				for _, subnet := range subnetList.Items {
					if subnet.Name == subnetName {
						hasSubnet = true
						break
					}
				}
				Expect(hasSubnet).To(BeTrue())
			})

			It("filters results by field selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetList, err := subnetManager.ListSubnets(ctx, client.MatchingFields{metav1.ObjectNameField: subnetName})
				Expect(err).NotTo(HaveOccurred())
				Expect(subnetList.Items).NotTo(BeEmpty())

				hasSubnet := false
				for _, subnet := range subnetList.Items {
					if subnet.Name == subnetName {
						hasSubnet = true
						break
					}
				}
				Expect(hasSubnet).To(BeTrue())
			})
		})

		Describe("AllocateEmptyIPPool", func() {
			var podT *corev1.Pod
			var podController types.PodTopController
			var podSelector *metav1.LabelSelector
			var ipNum int
			var ifName string
			BeforeEach(func() {
				ipNum = 1
				ifName = constant.ClusterDefaultInterfaceName
				podT = &corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: subnetName,
						UID:  uuid.NewUUID(),
					},
					Spec: corev1.PodSpec{
						NodeName: "node",
					},
				}
				podSelector = &metav1.LabelSelector{MatchLabels: podT.Labels}
			})

			It("inputs empty subnet name", func() {
				ctx := context.TODO()
				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, "", podController, podSelector, ipNum, constant.IPv4, true, ifName)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
				Expect(subnetIPPool).To(BeNil())
			})

			It("The value of input ipNum is less than 0 ", func() {
				ctx := context.TODO()
				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, -1, constant.IPv4, true, ifName)
				Expect(err).To(MatchError(constant.ErrWrongInput))
				Expect(subnetIPPool).To(BeNil())
			})

			It("inputs non-existent Subnet", func() {
				ctx := context.TODO()
				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, constant.IPv4, true, ifName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(subnetIPPool).To(BeNil())
			})

			It("avoids modifying the terminating Subnet", func() {
				now := metav1.Now()
				subnetT.SetDeletionTimestamp(&now)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, constant.IPv4, true, ifName)
				Expect(err).To(MatchError(constant.ErrWrongInput))
				Expect(subnetIPPool).To(BeNil())
			})

			It("inputs IP's Version is IPv4", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, constant.IPv4, true, ifName)
				Expect(err).NotTo(HaveOccurred())
				Expect(subnetIPPool).NotTo(BeNil())
				Expect(*subnetIPPool.Spec.IPVersion).To(Equal(constant.IPv4))
			})

			It("inputs IP's Version is IPv6", func() {
				ctx := context.TODO()
				ipVersion := constant.IPv6
				subnet := "abcd:1234::/120"
				subnetT.Spec.IPVersion = &ipVersion
				subnetT.Spec.Subnet = subnet
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, ipVersion, true, ifName)
				Expect(err).NotTo(HaveOccurred())
				Expect(subnetIPPool).NotTo(BeNil())
				Expect(*subnetIPPool.Spec.IPVersion).To(Equal(constant.IPv6))
			})

			It("failed to set owner reference due to some unknown errors", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyFuncReturn(controllerutil.SetControllerReference, constant.ErrUnknown)
				defer patches.Reset()

				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, constant.IPv4, true, ifName)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(subnetIPPool).To(BeNil())
			})

			It("failed to create IPPool in subnet due to some unknown errors", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyMethodReturn(fakeClient, "Create", constant.ErrUnknown)
				defer patches.Reset()

				subnetIPPool, err := subnetManager.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, constant.IPv4, true, ifName)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(subnetIPPool).To(BeNil())
			})
		})

		Describe("CheckScaleIPPool", func() {
			var ipNum int
			var ipPoolName string
			var ipPoolT *spiderpoolv1.SpiderIPPool
			var labels map[string]string

			BeforeEach(func() {
				ipNum = 1
				ipPoolName = fmt.Sprintf("IPPool-%v", count)
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

			It("inputs nil IPPool", func() {
				ctx := context.TODO()
				err := subnetManager.CheckScaleIPPool(ctx, nil, subnetName, ipNum)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("The value of input ipNum is less than 0 ", func() {
				ctx := context.TODO()
				err := subnetManager.CheckScaleIPPool(ctx, ipPoolT, subnetName, -1)
				Expect(err).To(MatchError(constant.ErrWrongInput))
			})

			It("inputs nil AutoDesiredIPCount", func() {
				ipPoolT.Status.AutoDesiredIPCount = nil
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = subnetManager.CheckScaleIPPool(ctx, ipPoolT, subnetName, ipNum)
				Expect(err).NotTo(HaveOccurred())

				ippool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ippool).NotTo(BeNil())
				Expect(*ippool.Status.AutoDesiredIPCount).To(Equal(int64(ipNum)))
			})

			It("inputs not nil AutoDesiredIPCount, For example 5", func() {
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(5)
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = subnetManager.CheckScaleIPPool(ctx, ipPoolT, subnetName, ipNum)
				Expect(err).NotTo(HaveOccurred())

				ippool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ippool).NotTo(BeNil())
				Expect(*ippool.Status.AutoDesiredIPCount).To(Equal(int64(ipNum)))
			})
		})
	})
})
