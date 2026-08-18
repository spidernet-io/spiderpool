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
	// metadataStatus builds a node-level ({"scope":"node-1"}) metadata
	// payload; callers must pin the pool via spec.nodeName = ["node-1"].
	metadataStatus := func(entries map[string]spiderpoolv2beta1.IPMetadataEntry, observedGeneration int64) *spiderpoolv2beta1.IPMetaData {
		payload := map[string]interface{}{
			"scope": "node-1",
			"ips":   entries,
		}
		data, err := json.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())
		raw := string(data)
		return &spiderpoolv2beta1.IPMetaData{
			Metadata:           &raw,
			ObservedGeneration: ptr.To(observedGeneration),
		}
	}
	syncMetadataCache := func(pool *spiderpoolv2beta1.SpiderIPPool) {
		Expect(ippoolmanager.SyncIPMetadataCache(ipPoolManager, pool)).To(Succeed())
	}

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

			Describe("IaaS prewarm ipMetaData gating (intersection model)", func() {
				BeforeEach(func() {
					mockRIPManager.EXPECT().
						AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
						Return(nil, nil).
						AnyTimes()

					ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.50.0/24"
					// spec.ips is populated normally for an IaaS pool (it is
					// NOT left empty and is NOT a separate address space --
					// data-model.md §1.1/§1.3).
					ipPoolT.Spec.IPs = []string{"172.18.50.10-172.18.50.20"}
					ipPoolT.Spec.NodeName = []string{"node-1"}
					if ipPoolT.Labels == nil {
						ipPoolT.Labels = map[string]string{}
					}
					ipPoolT.Labels[constant.LabelIPPoolIaasProvider] = "huaweicloud"
				})

				It("selects the address present in both the spec.ips candidate set and ipMetaData.metadata, skipping keys outside spec.ips/claimed/malformed, and honors ascending order", func() {
					// .05 is outside spec.ips entirely -- must be ignored
					// without needing a separate "well-formed relative to
					// spec.ips" validation step (data-model.md §1.3).
					// "not-an-ip" is a malformed map key.
					// .13 is claimed via status.allocatedIPs already.
					// .11 and .20 are both valid, unclaimed, in-range
					// candidates; ascending order must select .11 first.
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.50.5":  {},
						"not-an-ip":    {},
						"172.18.50.13": {},
						"172.18.50.20": {},
						"172.18.50.11": {},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					records := spiderpoolv2beta1.PoolIPAllocations{
						"172.18.50.13": spiderpoolv2beta1.PoolIPAllocation{NamespacedName: "default/other", PodUID: "other-uid"},
					}
					data, err := convert.MarshalIPPoolAllocatedIPs(records)
					Expect(err).NotTo(HaveOccurred())
					ipPoolT.Status.AllocatedIPs = data
					ipPoolT.Status.AllocatedIPCount = ptr.To(int64(1))

					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromMetadata).To(BeTrue())
					Expect(*res.Address).To(Equal("172.18.50.11/24"))
				})

				It("copies the matched entry's MAC/VLAN onto the resulting IPConfig", func() {
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.50.15": {MAC: "fa:16:3e:aa:bb:cc", VLAN: ptr.To(int32(2014))},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromMetadata).To(BeTrue())
					Expect(*res.Address).To(Equal("172.18.50.15/24"))
					Expect(res.Mac).To(Equal("fa:16:3e:aa:bb:cc"))
					Expect(res.Vlan).To(Equal(int64(2014)))
				})

				It("returns ErrIPUsedOut, not a static fallback allocation, for a freshly-created IaaS pool with no metadata entries yet", func() {
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(constant.ErrIPUsedOut))
					Expect(fromMetadata).To(BeFalse())
					Expect(res).To(BeNil())
				})

				It("fails closed when the provider has not observed the current generation", func() {
					ipPoolT.Generation = 7
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.50.11": {},
					}, 6)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(ContainSubstring(constant.ErrIPMetadataNotReady.Error())))
					Expect(fromMetadata).To(BeFalse())
					Expect(res).To(BeNil())
				})

				It("fails closed when current-generation metadata JSON is malformed", func() {
					raw := "{"
					ipPoolT.Status.IPMetaData = &spiderpoolv2beta1.IPMetaData{
						Metadata:           &raw,
						ObservedGeneration: ptr.To(ipPoolT.Generation),
					}
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(ContainSubstring(constant.ErrIPMetadataNotReady.Error())))
					Expect(fromMetadata).To(BeFalse())
					Expect(res).To(BeNil())
				})

				It("returns ErrIPUsedOut when ipMetaData.metadata is non-empty but no key falls inside the spec.ips candidate set", func() {
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.50.99": {}, // outside spec.ips
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(constant.ErrIPUsedOut))
					Expect(fromMetadata).To(BeFalse())
					Expect(res).To(BeNil())
				})

				It("is entirely unaffected by ipMetaData content when the pool does not carry the iaas-provider label", func() {
					delete(ipPoolT.Labels, constant.LabelIPPoolIaasProvider)
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.50.20": {},
					}, ipPoolT.Generation)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromMetadata).To(BeFalse())
					// Falls through to the ordinary ascending spec.ips
					// candidate order, ignoring the metadata entirely.
					Expect(*res.Address).To(Equal("172.18.50.10/24"))
				})
			})

			Describe("Global IaaS pool allocation (realtime + sticky sub-ENI cache)", func() {
				globalMetadataStatus := func(entries map[string]spiderpoolv2beta1.IPMetadataEntry, observedGeneration int64) *spiderpoolv2beta1.IPMetaData {
					payload := map[string]interface{}{
						"scope":     "",
						"parentNic": "eth0",
						"ips":       entries,
					}
					data, err := json.Marshal(payload)
					Expect(err).NotTo(HaveOccurred())
					raw := string(data)
					return &spiderpoolv2beta1.IPMetaData{
						Metadata:           &raw,
						ObservedGeneration: ptr.To(observedGeneration),
					}
				}

				BeforeEach(func() {
					mockRIPManager.EXPECT().
						AssembleReservedIPs(gomock.Eq(ctx), gomock.Eq(constant.IPv4)).
						Return(nil, nil).
						AnyTimes()

					ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.70.0/24"
					ipPoolT.Spec.IPs = []string{"172.18.70.10-172.18.70.20"}
					// Global mode: IaaS label present, spec.nodeName EMPTY.
					ipPoolT.Spec.NodeName = nil
					if ipPoolT.Labels == nil {
						ipPoolT.Labels = map[string]string{}
					}
					ipPoolT.Labels[constant.LabelIPPoolIaasProvider] = "huaweicloud"
					podT.Spec.NodeName = "node-1"
				})

				It("returns a zero-RPC hit only for an entry bound to the local node with a trustworthy VLAN", func() {
					ipPoolT.Status.IPMetaData = globalMetadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.70.10": {MAC: "fa:16:3e:00:00:10", VLAN: ptr.To(int32(2010)), Node: ptr.To("node-2")},
						"172.18.70.12": {MAC: "fa:16:3e:00:00:12", VLAN: ptr.To(int32(2012)), Node: ptr.To("node-1")},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromMetadata).To(BeTrue())
					// .10 is bound to node-2 -> not a hit; .12 is the local hit.
					Expect(*res.Address).To(Equal("172.18.70.12/24"))
					Expect(res.Mac).To(Equal("fa:16:3e:00:00:12"))
					Expect(res.Vlan).To(Equal(int64(2012)))
				})

				It("falls back to the cold path on a cache miss, preferring unbound addresses and skipping detaching entries", func() {
					ipPoolT.Status.IPMetaData = globalMetadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						// Detaching (node present, vlan == -1): never allocatable.
						"172.18.70.10": {MAC: "fa:16:3e:00:00:10", VLAN: ptr.To(int32(-1)), Node: ptr.To("node-2")},
						// Idle on another node: tier-2 steal candidate only.
						"172.18.70.11": {MAC: "fa:16:3e:00:00:11", VLAN: ptr.To(int32(2011)), Node: ptr.To("node-2")},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					// Cold path: the caller must run the provider Allocate RPC.
					Expect(fromMetadata).To(BeFalse())
					// Tier 1 unbound (.12, no entry) beats the tier-2 steal
					// (.11) and the detaching entry (.10) is skipped.
					Expect(*res.Address).To(Equal("172.18.70.12/24"))
				})

				It("steals the lowest idle-on-another-node address when no unbound address remains", func() {
					entries := map[string]spiderpoolv2beta1.IPMetadataEntry{}
					records := spiderpoolv2beta1.PoolIPAllocations{}
					for i := 10; i <= 20; i++ {
						addr := fmt.Sprintf("172.18.70.%d", i)
						entries[addr] = spiderpoolv2beta1.IPMetadataEntry{
							MAC: "fa:16:3e:00:00:aa", VLAN: ptr.To(int32(2000 + i)), Node: ptr.To("node-2"),
						}
						if i > 11 {
							records[addr] = spiderpoolv2beta1.PoolIPAllocation{NamespacedName: "default/other", PodUID: "other-uid"}
						}
					}
					ipPoolT.Status.IPMetaData = globalMetadataStatus(entries, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					data, err := convert.MarshalIPPoolAllocatedIPs(records)
					Expect(err).NotTo(HaveOccurred())
					ipPoolT.Status.AllocatedIPs = data
					ipPoolT.Status.AllocatedIPCount = ptr.To(int64(len(records)))
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromMetadata).To(BeFalse())
					Expect(*res.Address).To(Equal("172.18.70.10/24"))
				})

				It("returns ErrIPUsedOut when every candidate is detaching", func() {
					ipPoolT.Spec.IPs = []string{"172.18.70.10"}
					ipPoolT.Status.IPMetaData = globalMetadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.70.10": {MAC: "fa:16:3e:00:00:10", VLAN: ptr.To(int32(-1)), Node: ptr.To("node-2")},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(constant.ErrIPUsedOut))
					Expect(fromMetadata).To(BeFalse())
					Expect(res).To(BeNil())
				})

				It("fails closed on a v2 payload whose global scope contradicts a node-pinned spec", func() {
					ipPoolT.Spec.NodeName = []string{"node-1"}
					ipPoolT.Status.IPMetaData = globalMetadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.70.10": {MAC: "fa:16:3e:00:00:10", VLAN: ptr.To(int32(2010))},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(ContainSubstring(constant.ErrIPMetadataNotReady.Error())))
					Expect(fromMetadata).To(BeFalse())
					Expect(res).To(BeNil())
				})
			})

			Describe("IaaS prewarm paired dual-stack allocation (pair-or-nothing, single-metadata-on-primary)", func() {
				var v6PoolName string
				var v6PoolT *spiderpoolv2beta1.SpiderIPPool

				BeforeEach(func() {
					mockRIPManager.EXPECT().
						AssembleReservedIPs(gomock.Any(), gomock.Any()).
						Return(nil, nil).
						AnyTimes()

					ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.60.0/24"
					ipPoolT.Spec.IPs = []string{"172.18.60.10-172.18.60.20"}
					ipPoolT.Spec.NodeName = []string{"node-1"}
					if ipPoolT.Labels == nil {
						ipPoolT.Labels = map[string]string{}
					}
					ipPoolT.Labels[constant.LabelIPPoolIaasProvider] = "huaweicloud"

					v6PoolName = ipPoolName + "-v6"
					if ipPoolT.Annotations == nil {
						ipPoolT.Annotations = map[string]string{}
					}
					ipPoolT.Annotations[constant.AnnoIPPoolPairPool] = v6PoolName

					// The metadata lives ONLY on the primary (v4) pool per
					// the single-metadata-on-primary-pool model
					// (contracts/spiderippool-iaas-extension.md); the v6
					// sibling never carries its own ipMetaData. Note the
					// deliberately OUT-OF-ORDER v4->v6 mapping (the reviewer
					// case): sorting v4 keys and v6 values independently
					// would pair .10 with ::10, mixing two entries.
					ipPoolT.Status.IPMetaData = metadataStatus(map[string]spiderpoolv2beta1.IPMetadataEntry{
						"172.18.60.10": {IPv6: ptr.To("fd00:60::20"), MAC: "fa:16:3e:aa:bb:cc", VLAN: ptr.To(int32(2014))},
						"172.18.60.20": {IPv6: ptr.To("fd00:60::10"), MAC: "fa:16:3e:dd:ee:ff", VLAN: ptr.To(int32(2015))},
					}, ipPoolT.Generation)
					syncMetadataCache(ipPoolT)

					v6PoolT = &spiderpoolv2beta1.SpiderIPPool{
						TypeMeta: ipPoolT.TypeMeta,
						ObjectMeta: metav1.ObjectMeta{
							Name: v6PoolName,
							Annotations: map[string]string{
								constant.AnnoIPPoolPairPool: ipPoolName,
							},
							Labels: map[string]string{
								constant.LabelIPPoolIaasProvider: "huaweicloud",
							},
						},
						Spec: spiderpoolv2beta1.IPPoolSpec{
							IPVersion: ptr.To(constant.IPv6),
							Subnet:    "fd00:60::/120",
							IPs:       []string{"fd00:60::10-fd00:60::20"},
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

				createBothPools := func() {
					Expect(fakeClient.Create(ctx, ipPoolT)).To(Succeed())
					Expect(tracker.Add(ipPoolT)).To(Succeed())
					Expect(fakeClient.Create(ctx, v6PoolT)).To(Succeed())
					Expect(tracker.Add(v6PoolT)).To(Succeed())
				}

				It("allocates both families of ONE metadata entry, never mixing entries even with out-of-order v4/v6 mappings, and records both sides in the two pools' statuses", func() {
					createBothPools()

					v4Res, v6Res, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())

					// Lowest v4 key wins; the v6 address MUST be that same
					// entry's ipv6 value (::20), NOT the numerically lowest
					// v6 address (::10) which belongs to another entry.
					Expect(*v4Res.Address).To(Equal("172.18.60.10/24"))
					Expect(*v6Res.Address).To(Equal("fd00:60::20/120"))
					Expect(v4Res.IPPool).To(Equal(ipPoolName))
					Expect(v6Res.IPPool).To(Equal(v6PoolName))
					Expect(v4Res.Mac).To(Equal("fa:16:3e:aa:bb:cc"))
					Expect(v6Res.Mac).To(Equal("fa:16:3e:aa:bb:cc"))
					Expect(v4Res.Vlan).To(Equal(int64(2014)))
					Expect(v6Res.Vlan).To(Equal(int64(2014)))

					// Both pools' statuses record their own side. Status
					// writes land on fakeClient (the writer), so verify there.
					var fetchedV4, fetchedV6 spiderpoolv2beta1.SpiderIPPool
					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolName}, &fetchedV4)).To(Succeed())
					v4Records, err := convert.UnmarshalIPPoolAllocatedIPs(fetchedV4.Status.AllocatedIPs)
					Expect(err).NotTo(HaveOccurred())
					Expect(v4Records).To(HaveKey("172.18.60.10"))

					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: v6PoolName}, &fetchedV6)).To(Succeed())
					v6Records, err := convert.UnmarshalIPPoolAllocatedIPs(fetchedV6.Status.AllocatedIPs)
					Expect(err).NotTo(HaveOccurred())
					Expect(v6Records).To(HaveKey("fd00:60::20"))
				})

				It("skips an entry whose v6 side is already claimed in the sibling pool and selects the next fully-available entry", func() {
					records := spiderpoolv2beta1.PoolIPAllocations{
						"fd00:60::20": spiderpoolv2beta1.PoolIPAllocation{NamespacedName: "default/other", PodUID: "other-uid"},
					}
					data, err := convert.MarshalIPPoolAllocatedIPs(records)
					Expect(err).NotTo(HaveOccurred())
					v6PoolT.Status.AllocatedIPs = data
					v6PoolT.Status.AllocatedIPCount = ptr.To(int64(1))
					createBothPools()

					v4Res, v6Res, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					// Entry .10<->::20 is unusable (its v6 is taken), so the
					// whole entry is skipped: the pair comes from .20<->::10.
					Expect(*v4Res.Address).To(Equal("172.18.60.20/24"))
					Expect(*v6Res.Address).To(Equal("fd00:60::10/120"))
				})

				It("skips an entry whose ipv6 value is not part of the sibling pool's own spec.ips", func() {
					v6PoolT.Spec.IPs = []string{"fd00:60::10"} // excludes fd00:60::20
					createBothPools()

					v4Res, v6Res, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(*v4Res.Address).To(Equal("172.18.60.20/24"))
					Expect(*v6Res.Address).To(Equal("fd00:60::10/120"))
				})

				It("returns ErrIPUsedOut when no entry is fully available on both sides", func() {
					v6PoolT.Spec.IPs = []string{"fd00:60::99"} // matches no entry
					createBothPools()

					v4Res, v6Res, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(constant.ErrIPUsedOut))
					Expect(v4Res).To(BeNil())
					Expect(v6Res).To(BeNil())

					// Nothing was committed to either pool.
					var fetchedV4, fetchedV6 spiderpoolv2beta1.SpiderIPPool
					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolName}, &fetchedV4)).To(Succeed())
					Expect(fetchedV4.Status.AllocatedIPs).To(BeNil())
					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: v6PoolName}, &fetchedV6)).To(Succeed())
					Expect(fetchedV6.Status.AllocatedIPs).To(BeNil())
				})

				It("converges on the SAME entry via the Pod-UID fast path when the v4 side was already committed by a previous round", func() {
					key, err := cache.MetaNamespaceKeyFunc(podT)
					Expect(err).NotTo(HaveOccurred())
					// Simulate a half-committed pair: only the v4 side of
					// entry .20<->::10 was written before an interruption.
					records := spiderpoolv2beta1.PoolIPAllocations{
						"172.18.60.20": spiderpoolv2beta1.PoolIPAllocation{NamespacedName: key, PodUID: string(podT.UID)},
					}
					data, err := convert.MarshalIPPoolAllocatedIPs(records)
					Expect(err).NotTo(HaveOccurred())
					ipPoolT.Status.AllocatedIPs = data
					ipPoolT.Status.AllocatedIPCount = ptr.To(int64(1))
					createBothPools()

					v4Res, v6Res, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					// The fast path must complete THIS entry's v6 side, even
					// though entry .10<->::20 sorts first among fresh picks.
					Expect(*v4Res.Address).To(Equal("172.18.60.20/24"))
					Expect(*v6Res.Address).To(Equal("fd00:60::10/120"))

					var fetchedV6 spiderpoolv2beta1.SpiderIPPool
					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: v6PoolName}, &fetchedV6)).To(Succeed())
					v6Records, err := convert.UnmarshalIPPoolAllocatedIPs(fetchedV6.Status.AllocatedIPs)
					Expect(err).NotTo(HaveOccurred())
					Expect(v6Records).To(HaveKey("fd00:60::10"))
				})

				It("never adopts a foreign v6 record on the fast path: a pair whose v6 side is owned by another Pod fails with ErrIPUsedOut", func() {
					key, err := cache.MetaNamespaceKeyFunc(podT)
					Expect(err).NotTo(HaveOccurred())
					// This Pod owns the v4 side of entry .20<->::10, but the
					// entry's v6 address was taken by ANOTHER Pod between
					// rounds (e.g. an out-of-band standalone allocation).
					v4Data, err := convert.MarshalIPPoolAllocatedIPs(spiderpoolv2beta1.PoolIPAllocations{
						"172.18.60.20": spiderpoolv2beta1.PoolIPAllocation{NamespacedName: key, PodUID: string(podT.UID)},
					})
					Expect(err).NotTo(HaveOccurred())
					ipPoolT.Status.AllocatedIPs = v4Data
					ipPoolT.Status.AllocatedIPCount = ptr.To(int64(1))

					v6Data, err := convert.MarshalIPPoolAllocatedIPs(spiderpoolv2beta1.PoolIPAllocations{
						"fd00:60::10": spiderpoolv2beta1.PoolIPAllocation{NamespacedName: "default/other", PodUID: "other-uid"},
					})
					Expect(err).NotTo(HaveOccurred())
					v6PoolT.Status.AllocatedIPs = v6Data
					v6PoolT.Status.AllocatedIPCount = ptr.To(int64(1))
					createBothPools()

					v4Res, v6Res, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(constant.ErrIPUsedOut))
					Expect(v4Res).To(BeNil())
					Expect(v6Res).To(BeNil())

					// The foreign v6 record is untouched (not adopted).
					var fetchedV6 spiderpoolv2beta1.SpiderIPPool
					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: v6PoolName}, &fetchedV6)).To(Succeed())
					v6Records, err := convert.UnmarshalIPPoolAllocatedIPs(fetchedV6.Status.AllocatedIPs)
					Expect(err).NotTo(HaveOccurred())
					Expect(v6Records["fd00:60::10"].PodUID).To(Equal("other-uid"))
				})

				It("global mode: allocates a cold-path pair with a fresh non-sticky v6 when no local cache hit exists", func() {
					// Switch the primary pool's metadata to schema v2 global
					// scope: one sticky pair bound to another node locks its
					// v6 (fd00:60::20) per FR-024 even though nothing claims
					// it in the sibling pool's allocatedIPs.
					payload := map[string]interface{}{
						"scope":     "",
						"parentNic": "eth0",
						"ips": map[string]spiderpoolv2beta1.IPMetadataEntry{
							"172.18.60.10": {IPv6: ptr.To("fd00:60::20"), MAC: "fa:16:3e:aa:bb:cc", VLAN: ptr.To(int32(2014)), Node: ptr.To("node-2")},
						},
					}
					data, err := json.Marshal(payload)
					Expect(err).NotTo(HaveOccurred())
					raw := string(data)
					ipPoolT.Status.IPMetaData = &spiderpoolv2beta1.IPMetaData{
						Metadata:           &raw,
						ObservedGeneration: ptr.To(ipPoolT.Generation),
					}
					ipPoolT.Spec.NodeName = nil
					syncMetadataCache(ipPoolT)
					podT.Spec.NodeName = "node-1"
					createBothPools()

					v4Res, v6Res, fromPair, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					// Cold path: the caller must run the provider RPC.
					Expect(fromPair).To(BeFalse())
					// v4 tier 1 unbound: .11 (no entry) beats stealing .10.
					Expect(*v4Res.Address).To(Equal("172.18.60.11/24"))
					// v6: lowest available NOT locked by a sticky pair --
					// ::10 is free, ::20 is excluded by the entry's ipv6.
					Expect(*v6Res.Address).To(Equal("fd00:60::10/120"))
				})

				It("global mode: cold path reuses the lifetime-sticky v6 of a created-but-detached sub-ENI instead of pairing a fresh one", func() {
					// A detached sub-ENI (no node) keeps its sticky v6; the
					// provider's Allocate RPC will re-attach it and return
					// that same v6, so selection must reuse fd00:60::20.
					payload := map[string]interface{}{
						"scope":     "",
						"parentNic": "eth0",
						"ips": map[string]spiderpoolv2beta1.IPMetadataEntry{
							"172.18.60.10": {IPv6: ptr.To("fd00:60::20"), MAC: "fa:16:3e:aa:bb:cc", VLAN: ptr.To(int32(-1))},
						},
					}
					data, err := json.Marshal(payload)
					Expect(err).NotTo(HaveOccurred())
					raw := string(data)
					ipPoolT.Status.IPMetaData = &spiderpoolv2beta1.IPMetaData{
						Metadata:           &raw,
						ObservedGeneration: ptr.To(ipPoolT.Generation),
					}
					ipPoolT.Spec.NodeName = nil
					syncMetadataCache(ipPoolT)
					podT.Spec.NodeName = "node-1"
					createBothPools()

					v4Res, v6Res, fromPair, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					// Cold path: the caller must run the provider RPC.
					Expect(fromPair).To(BeFalse())
					// Tier 1 unbound: the detached entry's v4 sorts first.
					Expect(*v4Res.Address).To(Equal("172.18.60.10/24"))
					// The sticky v6 is reused, not a fresh ::10.
					Expect(*v6Res.Address).To(Equal("fd00:60::20/120"))
				})

				It("global mode: hits the sticky pair bound to the local node without any provider call", func() {
					payload := map[string]interface{}{
						"scope":     "",
						"parentNic": "eth0",
						"ips": map[string]spiderpoolv2beta1.IPMetadataEntry{
							"172.18.60.10": {IPv6: ptr.To("fd00:60::20"), MAC: "fa:16:3e:aa:bb:cc", VLAN: ptr.To(int32(2014)), Node: ptr.To("node-1")},
						},
					}
					data, err := json.Marshal(payload)
					Expect(err).NotTo(HaveOccurred())
					raw := string(data)
					ipPoolT.Status.IPMetaData = &spiderpoolv2beta1.IPMetaData{
						Metadata:           &raw,
						ObservedGeneration: ptr.To(ipPoolT.Generation),
					}
					ipPoolT.Spec.NodeName = nil
					syncMetadataCache(ipPoolT)
					podT.Spec.NodeName = "node-1"
					createBothPools()

					v4Res, v6Res, fromPair, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromPair).To(BeTrue())
					Expect(*v4Res.Address).To(Equal("172.18.60.10/24"))
					Expect(*v6Res.Address).To(Equal("fd00:60::20/120"))
					Expect(v4Res.Mac).To(Equal("fa:16:3e:aa:bb:cc"))
					Expect(v4Res.Vlan).To(Equal(int64(2014)))
				})

				It("rejects AllocateIPPair on a pool that is not a paired IaaS v4 primary pool", func() {
					delete(ipPoolT.Annotations, constant.AnnoIPPoolPairPool)
					createBothPools()

					_, _, _, err := ipPoolManager.AllocateIPPair(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).To(MatchError(ContainSubstring("not a paired IaaS v4 primary pool")))
				})

				It("still serves a plain single-family AllocateIP from the primary pool's own metadata when only v4 is requested", func() {
					createBothPools()

					res, fromMetadata, err := ipPoolManager.AllocateIP(ctx, ipPoolName, nic, podT, spiderpooltypes.PodTopController{})
					Expect(err).NotTo(HaveOccurred())
					Expect(fromMetadata).To(BeTrue())
					Expect(*res.Address).To(Equal("172.18.60.10/24"))

					// The v6 pool was never touched.
					var fetchedV6 spiderpoolv2beta1.SpiderIPPool
					Expect(fakeClient.Get(ctx, types.NamespacedName{Name: v6PoolName}, &fetchedV6)).To(Succeed())
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
