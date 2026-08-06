// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
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
				GracePeriodSeconds: ptr.To(int64(0)),
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

		Describe("AllocateIP", func() {
			var nic string
			var podT *corev1.Pod

			BeforeEach(func() {
				nic = "eth0"
				podT = &corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod",
						Namespace: "default",
						UID:       uuid.NewUUID(),
					},
					Spec: corev1.PodSpec{},
				}
			})

			It("allocate IP address from non-existent IPPool", func() {
				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(res).To(BeNil())
			})

			It("failed to assemble the reserved IP addresses due to some unknown errors", func() {
				mockRIPManager.EXPECT().
					AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
					Return(nil, constant.ErrUnknown).
					Times(1)

				ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.40")

				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(res).To(BeNil())
			})

			It("failed to update IPPool due to some unknown errors", func() {
				mockRIPManager.EXPECT().
					AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
					Return(nil, nil).
					Times(1)

				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.40")

				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(res).To(BeNil())
			})

			It("runs out of retries to update IPPool, but conflicts still occur", func() {
				mockRIPManager.EXPECT().
					AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
					Return(nil, nil).
					Times(5)

				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.40")

				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
				Expect(res).To(BeNil())
			})

			It("allocate IP address with normal pod", func() {
				mockRIPManager.EXPECT().
					AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
					Return(nil, nil).
					Times(1)

				ipVersion := constant.IPv4
				allocatedIP := "172.18.40.40/24"
				gateway := "172.18.40.1"

				ip, ipNet, err := net.ParseCIDR(allocatedIP)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = ptr.To(ipVersion)
				ipPoolT.Spec.Subnet = ipNet.String()
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, ip.String())
				ipPoolT.Spec.Gateway = ptr.To(gateway)

				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
				Expect(err).NotTo(HaveOccurred())
				Expect(*res.Nic).To(Equal(nic))
				Expect(*res.Version).To(Equal(ipVersion))
				Expect(*res.Address).To(Equal(allocatedIP))
				Expect(res.IPPool).To(Equal(ipPoolT.Name))
				Expect(res.Gateway).To(Equal(gateway))
			})

			It("allocate IP address with kubevirt vm pod", func() {
				mockRIPManager.EXPECT().
					AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
					Return(nil, nil).
					Times(1)

				ipVersion := constant.IPv4
				allocatedIP := "172.18.40.41/24"
				gateway := "172.18.40.1"
				vlan := int64(0)

				ip, ipNet, err := net.ParseCIDR(allocatedIP)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = ptr.To(ipVersion)
				ipPoolT.Spec.Subnet = ipNet.String()
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, ip.String())
				ipPoolT.Spec.Gateway = ptr.To(gateway)

				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				podTopController := spiderpooltypes.PodTopController{
					AppNamespacedName: spiderpooltypes.AppNamespacedName{
						APIVersion: kubevirtv1.SchemeGroupVersion.String(),
						Kind:       constant.KindKubevirtVMI,
						Namespace:  "default",
						Name:       "vmi-demo",
					},
					UID: uuid.NewUUID(),
					APP: nil,
				}
				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, podTopController)
				Expect(err).NotTo(HaveOccurred())
				Expect(*res.Nic).To(Equal(nic))
				Expect(*res.Version).To(Equal(ipVersion))
				Expect(*res.Address).To(Equal(allocatedIP))
				Expect(res.IPPool).To(Equal(ipPoolT.Name))
				Expect(res.Gateway).To(Equal(gateway))
				Expect(res.Vlan).To(Equal(vlan))
			})

			It("allocate IP address from the previous records", func() {
				mockRIPManager.EXPECT().
					AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
					Return(nil, nil).
					Times(1)

				ipVersion := constant.IPv4
				allocatedIP := "172.18.40.40/24"
				gateway := "172.18.40.1"
				vlan := int64(0)

				ip, ipNet, err := net.ParseCIDR(allocatedIP)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = ptr.To(ipVersion)
				ipPoolT.Spec.Subnet = ipNet.String()
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, ip.String())
				ipPoolT.Spec.Gateway = ptr.To(gateway)

				key, err := cache.MetaNamespaceKeyFunc(podT)
				Expect(err).NotTo(HaveOccurred())

				records := spiderpoolv2beta1.PoolIPAllocations{
					ip.String(): spiderpoolv2beta1.PoolIPAllocation{
						NamespacedName: key,
						PodUID:         string(podT.UID),
					},
				}
				allocatedIPs, err := json.Marshal(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status = spiderpoolv2beta1.IPPoolStatus{
					AllocatedIPs:     ptr.To(string(allocatedIPs)),
					TotalIPCount:     ptr.To(int64(1)),
					AllocatedIPCount: ptr.To(int64(1)),
				}

				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				res, _, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
				Expect(err).NotTo(HaveOccurred())
				Expect(*res.Nic).To(Equal(nic))
				Expect(*res.Version).To(Equal(ipVersion))
				Expect(*res.Address).To(Equal(allocatedIP))
				Expect(res.IPPool).To(Equal(ipPoolT.Name))
				Expect(res.Gateway).To(Equal(gateway))
				Expect(res.Vlan).To(Equal(vlan))
			})

			Describe("IaaS prewarm ledger gating", func() {
				BeforeEach(func() {
					mockRIPManager.EXPECT().
						AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
						Return(nil, nil).
						AnyTimes()

					ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.50.0/24"
				})

				It("only selects a Ready, unclaimed entry, skipping NotReady/Releasing/claimed/malformed entries", func() {
					ipPoolT.Status.IaasIPs = []spiderpoolv2beta1.IaasIPAllocation{
						{IPv4: ptr.To("172.18.50.10"), Phase: constant.IaasIPAllocationPhaseNotReady},
						{IPv4: ptr.To("172.18.50.11"), Phase: constant.IaasIPAllocationPhaseReleasing},
						{Phase: constant.IaasIPAllocationPhaseReady}, // malformed: no address
						{IPv4: ptr.To("172.18.50.13"), Phase: constant.IaasIPAllocationPhaseReady},
					}
					// Mark .13 as already claimed via status.allocatedIPs.
					records := spiderpoolv2beta1.PoolIPAllocations{
						"172.18.50.13": spiderpoolv2beta1.PoolIPAllocation{NamespacedName: "default/other", PodUID: "other-uid"},
					}
					data, err := convert.MarshalIPPoolAllocatedIPs(records)
					Expect(err).NotTo(HaveOccurred())
					ipPoolT.Status.AllocatedIPs = data
					ipPoolT.Status.AllocatedIPCount = ptr.To(int64(1))
					ipPoolT.Status.IaasIPs = append(ipPoolT.Status.IaasIPs, spiderpoolv2beta1.IaasIPAllocation{
						IPv4: ptr.To("172.18.50.20"), Phase: constant.IaasIPAllocationPhaseReady,
					})

					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromLedger, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromLedger).To(BeTrue())
					Expect(*res.Address).To(Equal("172.18.50.20/24"))
				})

				It("falls through unchanged when IaasIPs is empty", func() {
					ipPoolT.Spec.IPs = []string{"172.18.50.30"}
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromLedger, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromLedger).To(BeFalse())
					Expect(*res.Address).To(Equal("172.18.50.30/24"))
				})

				It("returns ErrIPUsedOut when IaasIPs is non-empty but no entry qualifies", func() {
					ipPoolT.Status.IaasIPs = []spiderpoolv2beta1.IaasIPAllocation{
						{IPv4: ptr.To("172.18.50.40"), Phase: constant.IaasIPAllocationPhaseNotReady},
					}
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromLedger, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(constant.ErrIPUsedOut))
					Expect(fromLedger).To(BeFalse())
					Expect(res).To(BeNil())
				})
			})

			Describe("IaaS prewarm ledger atomic pairing and single-stack behavior", func() {
				var v6PoolName string
				var v6PoolT *spiderpoolv2beta1.SpiderIPPool

				BeforeEach(func() {
					mockRIPManager.EXPECT().
						AssembleReservedIPs(gomock.Any(), gomock.Any()).
						Return(nil, nil).
						AnyTimes()

					ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.60.0/24"
					ipPoolT.Status.IaasIPs = []spiderpoolv2beta1.IaasIPAllocation{
						{IPv4: ptr.To("172.18.60.10"), IPv6: ptr.To("fd00:60::10"), Phase: constant.IaasIPAllocationPhaseReady},
					}

					v6PoolName = ipPoolName + "-v6"
					v6PoolT = &spiderpoolv2beta1.SpiderIPPool{
						TypeMeta: ipPoolT.TypeMeta,
						ObjectMeta: metav1.ObjectMeta{
							Name: v6PoolName,
						},
						Spec: spiderpoolv2beta1.IPPoolSpec{
							IPVersion: ptr.To(constant.IPv6),
							Subnet:    "fd00:60::/120",
						},
						Status: spiderpoolv2beta1.IPPoolStatus{
							IaasIPs: []spiderpoolv2beta1.IaasIPAllocation{
								{IPv4: ptr.To("172.18.60.10"), IPv6: ptr.To("fd00:60::10"), Phase: constant.IaasIPAllocationPhaseReady},
							},
						},
					}
				})

				AfterEach(func() {
					// NOTE: this AfterEach runs before the outer AfterEach
					// (Ginkgo runs AfterEach nodes innermost-first), so the
					// outer-scope deleteOption is not yet assigned here.
					// Build delete options locally instead of relying on it.
					policy := metav1.DeletePropagationForeground
					localDeleteOption := &client.DeleteOptions{
						GracePeriodSeconds: ptr.To(int64(0)),
						PropagationPolicy:  &policy,
					}

					err := fakeClient.Delete(ctx, v6PoolT, localDeleteOption)
					Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
					err = tracker.Delete(
						schema.GroupVersionResource{
							Group:    constant.SpiderpoolAPIGroup,
							Version:  constant.SpiderpoolAPIVersion,
							Resource: "spiderippools",
						},
						v6PoolT.Namespace,
						v6PoolT.Name,
					)
					Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
				})

				It("allocates both addresses of a paired entry for a dual-stack request, from each pool's own ledger", func() {
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())
					Expect(fakeClient.Create(ctx, v6PoolT)).To(Succeed())
					Expect(tracker.Add(v6PoolT)).To(Succeed())

					v4Res, v4FromLedger, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(v4FromLedger).To(BeTrue())
					Expect(*v4Res.Address).To(Equal("172.18.60.10/24"))

					v6Res, v6FromLedger, err := ipPoolManager.AllocateIP(ctx, v6PoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(v6FromLedger).To(BeTrue())
					Expect(*v6Res.Address).To(Equal("fd00:60::10/120"))
				})

				It("allocates only the requested family for a single-stack request, leaving the sibling pool untouched", func() {
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())
					Expect(fakeClient.Create(ctx, v6PoolT)).To(Succeed())
					Expect(tracker.Add(v6PoolT)).To(Succeed())

					v4Res, v4FromLedger, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(v4FromLedger).To(BeTrue())
					Expect(*v4Res.Address).To(Equal("172.18.60.10/24"))

					// The v6 pool was never called; its ledger entry remains
					// fully available (not claimed).
					fetchedV6, err := ipPoolManager.GetIPPoolByName(ctx, v6PoolName, constant.IgnoreCache)
					Expect(err).NotTo(HaveOccurred())
					Expect(fetchedV6.Status.AllocatedIPs).To(BeNil())
				})
			})
		})

		Describe("ReleaseIP", func() {
			var ip string
			var uid string
			var records spiderpoolv2beta1.PoolIPAllocations

			BeforeEach(func() {
				ip = "172.18.40.40"
				uid = string(uuid.NewUUID())
				records = spiderpoolv2beta1.PoolIPAllocations{
					ip: spiderpoolv2beta1.PoolIPAllocation{
						NamespacedName: "default/pod",
						PodUID:         uid,
					},
				}
			})

			It("release IP record from non-existent IPPool", func() {
				err := ipPoolManager.ReleaseIP(ctx, ipPoolName, []spiderpooltypes.IPAndUID{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})

			It("release the IP record with unmatched Pod UID", func() {
				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, []spiderpooltypes.IPAndUID{{IP: ip, UID: string(uuid.NewUUID())}})
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update IPPool due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, []spiderpooltypes.IPAndUID{{IP: ip, UID: uid}})
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("runs out of retries to update IPPool, but conflicts still occur", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, []spiderpooltypes.IPAndUID{{IP: ip, UID: uid}})
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
			})

			It("release the IP record", func() {
				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, []spiderpooltypes.IPAndUID{{IP: ip, UID: uid}})
				Expect(err).NotTo(HaveOccurred())

				var ipPool spiderpoolv2beta1.SpiderIPPool
				err = fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolT.Name}, &ipPool)
				Expect(err).NotTo(HaveOccurred())

				newRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
				Expect(err).NotTo(HaveOccurred())
				Expect(newRecords).To(BeEmpty())
			})
		})

		Describe("UpdateAllocatedIPs", func() {
			var ip string
			var uid string
			var records spiderpoolv2beta1.PoolIPAllocations

			BeforeEach(func() {
				ip = "172.18.40.40"
				uid = string(uuid.NewUUID())
				records = spiderpoolv2beta1.PoolIPAllocations{
					ip: spiderpoolv2beta1.PoolIPAllocation{
						NamespacedName: "default/pod",
						PodUID:         uid,
					},
				}
			})

			It("updates the allocated IP record from non-existent IPPool", func() {
				err := ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, "default/pod", []spiderpooltypes.IPAndUID{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})

			It("updates the allocated IP record for Pod that have not been recreated", func() {
				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, "default/pod", []spiderpooltypes.IPAndUID{{IP: ip, UID: uid}})
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update IPPool due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, "default/pod", []spiderpooltypes.IPAndUID{{IP: ip, UID: string(uuid.NewUUID())}})
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("runs out of retries to update IPPool, but conflicts still occur", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, "default/pod", []spiderpooltypes.IPAndUID{{IP: ip, UID: string(uuid.NewUUID())}})
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
			})

			It("failed to update IPPool due to data broken", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, "default/abc", []spiderpooltypes.IPAndUID{{IP: ip, UID: string(uuid.NewUUID())}})
				Expect(err).To(HaveOccurred())
			})

			It("updates the allocated IP record", func() {
				data, err := convert.MarshalIPPoolAllocatedIPs(records)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Status.AllocatedIPs = data
				err = fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = tracker.Add(ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				newUID := string(uuid.NewUUID())
				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, "default/pod", []spiderpooltypes.IPAndUID{{IP: ip, UID: newUID}})
				Expect(err).NotTo(HaveOccurred())

				var ipPool spiderpoolv2beta1.SpiderIPPool
				err = fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolT.Name}, &ipPool)
				Expect(err).NotTo(HaveOccurred())

				newRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
				Expect(err).NotTo(HaveOccurred())
				Expect(newRecords[ip].PodUID).To(Equal(newUID))
			})
		})

		Describe("ParseWildcardPoolNameList", func() {
			It("standard IPPool names", func() {
				poolNamesArr := []string{"pool1", "pool2"}
				newPoolNames, hasWildcard, err := ipPoolManager.ParseWildcardPoolNameList(ctx, poolNamesArr, constant.IPv4)
				Expect(err).NotTo(HaveOccurred())
				Expect(hasWildcard).To(BeFalse())
				Expect(newPoolNames).To(Equal(poolNamesArr))
			})

			It("wildcard IPPool name", func() {
				ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				poolNamesArr := []string{"ippool*", "pp1"}
				newPoolNames, hasWildcard, err := ipPoolManager.ParseWildcardPoolNameList(ctx, poolNamesArr, constant.IPv4)
				Expect(err).NotTo(HaveOccurred())
				Expect(hasWildcard).To(BeTrue())
				Expect(newPoolNames).To(HaveLen(2))
				Expect(newPoolNames[0]).To(Equal(ipPoolName))
			})

			It("wildcard IPPool name with no matched", func() {
				ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				// this wildcard would not match any IPPools' name
				poolNamesArr := []string{"aaa*"}
				newPoolNames, hasWildcard, err := ipPoolManager.ParseWildcardPoolNameList(ctx, poolNamesArr, constant.IPv4)
				Expect(err).NotTo(HaveOccurred())
				Expect(hasWildcard).To(BeTrue())
				Expect(newPoolNames).To(HaveLen(0))
			})

			It("invalid wildcard", func() {
				ipPoolT.Spec.IPVersion = ptr.To(constant.IPv6)
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				poolNamesArr := []string{"p1", "pool*", "[ippool]["}
				_, _, err = ipPoolManager.ParseWildcardPoolNameList(ctx, poolNamesArr, constant.IPv6)
				Expect(err).To(HaveOccurred())
			})

			It("failed to call ListIPPools with wildcard usage", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				poolNamesArr := []string{"ippool*", "pp1"}
				_, _, err := ipPoolManager.ParseWildcardPoolNameList(ctx, poolNamesArr, constant.IPv4)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(constant.ErrUnknown))
			})
		})
	})
})
