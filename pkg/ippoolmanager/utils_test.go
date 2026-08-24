// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"net"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("IPPoolManager-utils", Label("ippool_manager_utils"), func() {
	Context("IsAutoCreatedIPPool", Labels{"unittest", "IsAutoCreatedIPPool"}, func() {
		It("normal IPPool", func() {
			var pool spiderpoolv2beta1.SpiderIPPool

			label := map[string]string{constant.LabelIPPoolOwnerApplicationName: "test-name"}
			pool.SetLabels(label)

			isAutoCreatedIPPool := IsAutoCreatedIPPool(&pool)
			Expect(isAutoCreatedIPPool).To(BeTrue())
		})

		It("auto-created IPPool", func() {
			var pool spiderpoolv2beta1.SpiderIPPool

			isAutoCreatedIPPool := IsAutoCreatedIPPool(&pool)
			Expect(isAutoCreatedIPPool).To(BeFalse())
		})
	})

	Context("Test Auto IPPool PodAffinity", Labels{"unittest", "AutoPool-PodAffinity"}, func() {
		It("match auto-created IPPool affinity", func() {
			podTopController := types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       constant.KindDeployment,
					Namespace:  "test-ns",
					Name:       "test-name",
				},
				UID: types2.UID("a-b-c"),
				APP: nil,
			}

			podAffinity := NewAutoPoolPodAffinity(podTopController)

			isMatchAutoPoolAffinity := IsMatchAutoPoolAffinity(podAffinity, podTopController)
			Expect(isMatchAutoPoolAffinity).To(BeTrue())
		})

		It("not match auto-created IPPool affinity", func() {
			podTopController := types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       constant.KindDeployment,
					Namespace:  "test-ns",
					Name:       "test-name",
				},
				UID: types2.UID("a-b-c"),
				APP: nil,
			}

			podAffinity := NewAutoPoolPodAffinity(podTopController)

			podTopController.Kind = constant.KindStatefulSet
			isMatchAutoPoolAffinity := IsMatchAutoPoolAffinity(podAffinity, podTopController)
			Expect(isMatchAutoPoolAffinity).To(BeFalse())
		})
	})

	Context("Test IPAM pool candidates order selections", func() {
		poolTemplate := &spiderpoolv2beta1.SpiderIPPool{}

		Context("IPPool with PodAffinity", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.PodAffinity = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"PodAffinityKey": "PodAffinityValue1",
					},
				}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		Context("IPPool with NodeAffinity", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.NodeAffinity = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"NodeAffinityKey": "NodeAffinityValue1",
					},
				}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		Context("IPPool with NamespaceAffinity", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.NamespaceAffinity = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"NamespaceAffinityKey": "NamespaceAffinityValue1",
					},
				}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		Context("IPPool with MultusName", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.MultusName = []string{"kube-system/macvlan"}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		It("no any affinities", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")

			byPoolPriority := ByPoolPriority{pool1, pool2}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
		})

		It("pool affinity priority in sequence", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool1.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}

			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")
			pool2.Spec.NodeName = []string{"master"}
			pool2.Spec.NodeAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"NodeAffinityKey1": "NodeAffinityValue1",
				},
			}

			pool3 := poolTemplate.DeepCopy()
			pool3.SetName("pool3")
			pool3.Spec.NamespaceName = []string{"kube-system"}
			pool3.Spec.NamespaceAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"NamespaceAffinityKey1": "NamespaceAffinityValue1",
				},
			}

			pool4 := poolTemplate.DeepCopy()
			pool4.SetName("pool4")
			pool4.Spec.MultusName = []string{"kube-system/macvlan"}

			pool5 := poolTemplate.DeepCopy()
			pool5.SetName("pool5")

			byPoolPriority := ByPoolPriority{pool1, pool2, pool3, pool4, pool5}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).Should(Equal(ByPoolPriority{pool1, pool2, pool3, pool4, pool5}))
		})

		It("pool affinity priority in multiple cases", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool1.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}

			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")
			pool2.Spec.NodeName = []string{"master"}

			pool3 := poolTemplate.DeepCopy()
			pool3.SetName("pool3")
			pool3.Spec.NodeName = []string{"master"}
			pool3.Spec.NamespaceName = []string{"kube-system"}

			pool4 := poolTemplate.DeepCopy()
			pool4.SetName("pool4")
			pool4.Spec.NamespaceName = []string{"kube-system"}
			pool4.Spec.MultusName = []string{"kube-system/macvlan"}

			byPoolPriority := ByPoolPriority{pool1, pool2, pool3, pool4}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).Should(Equal(ByPoolPriority{pool1, pool3, pool2, pool4}))
		})

		It("pool affinity priority in chaos cases", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool1.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}
			pool1.Spec.NamespaceName = []string{"kube-system"}
			pool1.Spec.MultusName = []string{"kube-system/macvlan"}

			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")
			pool2.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}
			pool2.Spec.NodeName = []string{"master"}

			byPoolPriority := ByPoolPriority{pool1, pool2}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).Should(Equal(ByPoolPriority{pool2, pool1}))
		})
	})

	Context("Test findAllocatedIPFromRecords", func() {
		var allocatedRecords spiderpoolv2beta1.PoolIPAllocations
		var ip, namespacedName, podUID string

		BeforeEach(func() {
			ip = "172.18.40.40"
			namespacedName = "default/testPod"
			podUID = string(uuid.NewUUID())
			allocatedRecords = spiderpoolv2beta1.PoolIPAllocations{
				ip: spiderpoolv2beta1.PoolIPAllocation{
					NamespacedName: namespacedName,
					PodUID:         podUID,
				},
			}
		})

		It("find previous allocated IP in the records", func() {
			_, hasFound := findAllocatedIPFromRecords(allocatedRecords, namespacedName, podUID)
			Expect(hasFound).To(BeTrue())
		})

		It("no previous allocated IP in the records", func() {
			_, hasFound := findAllocatedIPFromRecords(allocatedRecords, "kube-system/testPod1", string(uuid.NewUUID()))
			Expect(hasFound).To(BeFalse())
		})

		It("no previous allocated IP in records due to different NamespacedName", func() {
			_, hasFound := findAllocatedIPFromRecords(allocatedRecords, "kube-system/testPod1", podUID)
			Expect(hasFound).To(BeFalse())
		})

		It("no previous allocated IP in records due to different podUID", func() {
			_, hasFound := findAllocatedIPFromRecords(allocatedRecords, namespacedName, string(uuid.NewUUID()))
			Expect(hasFound).To(BeFalse())
		})
	})

	Context("Test Wildcard", func() {
		It("For single string variable", func() {
			hasWildcardInStr := HasWildcardInStr("pool")
			Expect(hasWildcardInStr).To(BeFalse())

			hasWildcardInStr = HasWildcardInStr("pool?")
			Expect(hasWildcardInStr).To(BeTrue())

			hasWildcardInStr = HasWildcardInStr("pool*")
			Expect(hasWildcardInStr).To(BeTrue())

			hasWildcardInStr = HasWildcardInStr("pool[abc]")
			Expect(hasWildcardInStr).To(BeTrue())
		})

		It("For string slice", func() {
			hasWildcardInSlice := HasWildcardInSlice([]string{"pool1", "pool2", "pool3"})
			Expect(hasWildcardInSlice).To(BeFalse())

			hasWildcardInSlice = HasWildcardInSlice([]string{"pool1", "v4pool*", "pool2"})
			Expect(hasWildcardInSlice).To(BeTrue())
		})
	})
})

var _ = Describe("Global pool helpers", Label("ippool_manager_utils"), func() {
	newIPs := func(addrs ...string) []net.IP {
		ips := make([]net.IP, 0, len(addrs))
		for _, a := range addrs {
			ips = append(ips, net.ParseIP(a))
		}
		return ips
	}

	Context("IsGlobalIaaSPool", Labels{"unittest", "IsGlobalIaaSPool"}, func() {
		It("requires the explicit iaas-global label with value \"true\"", func() {
			pool := &spiderpoolv2beta1.SpiderIPPool{}
			Expect(IsGlobalIaaSPool(pool)).To(BeFalse())

			// The iaas-provider label alone no longer implies a global pool.
			pool.SetLabels(map[string]string{constant.LabelIPPoolIaasProvider: "huaweicloud"})
			Expect(IsGlobalIaaSPool(pool)).To(BeFalse())

			pool.Labels[constant.LabelIPPoolIaasGlobal] = "true"
			Expect(IsGlobalIaaSPool(pool)).To(BeTrue())

			// spec.nodeName does not affect global-pool recognition.
			pool.Spec.NodeName = []string{"node-1"}
			Expect(IsGlobalIaaSPool(pool)).To(BeTrue())

			pool.Labels[constant.LabelIPPoolIaasGlobal] = "false"
			Expect(IsGlobalIaaSPool(pool)).To(BeFalse())

			// The marker works without the iaas-provider label.
			solo := &spiderpoolv2beta1.SpiderIPPool{}
			solo.SetLabels(map[string]string{constant.LabelIPPoolIaasGlobal: "true"})
			Expect(IsGlobalIaaSPool(solo)).To(BeTrue())
		})
	})

	Context("isGlobalCacheHitEntry", Labels{"unittest", "isGlobalCacheHitEntry"}, func() {
		It("hits only entries bound locally with a trustworthy VLAN", func() {
			local := &spiderpoolv2beta1.IPMetadataEntry{Node: ptr.To("node-1"), VLAN: ptr.To(int32(7))}
			Expect(isGlobalCacheHitEntry(local, "node-1")).To(BeTrue())

			other := &spiderpoolv2beta1.IPMetadataEntry{Node: ptr.To("node-2"), VLAN: ptr.To(int32(7))}
			Expect(isGlobalCacheHitEntry(other, "node-1")).To(BeFalse())

			unbound := &spiderpoolv2beta1.IPMetadataEntry{VLAN: ptr.To(int32(7))}
			Expect(isGlobalCacheHitEntry(unbound, "node-1")).To(BeFalse())

			// vlan == -1 is the provider detaching race guard (FR-023).
			detaching := &spiderpoolv2beta1.IPMetadataEntry{Node: ptr.To("node-1"), VLAN: ptr.To(int32(-1))}
			Expect(isGlobalCacheHitEntry(detaching, "node-1")).To(BeFalse())
		})
	})

	Context("FindReadyIPMetadata in global mode", Labels{"unittest", "FindReadyIPMetadataGlobal"}, func() {
		It("skips other-node, unbound, and detaching entries", func() {
			metadata := map[string]spiderpoolv2beta1.IPMetadataEntry{
				"10.0.0.1": {Node: ptr.To("node-2"), VLAN: ptr.To(int32(7))},
				"10.0.0.2": {Node: ptr.To("node-1"), VLAN: ptr.To(int32(-1))},
				"10.0.0.3": {VLAN: ptr.To(int32(7))},
				"10.0.0.4": {Node: ptr.To("node-1"), VLAN: ptr.To(int32(9))},
			}
			candidates := newIPs("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4")

			entry, addr, ok := FindReadyIPMetadata(metadata, types.IPVersion(4), candidates, "node-1", true)
			Expect(ok).To(BeTrue())
			Expect(addr).To(Equal("10.0.0.4"))
			Expect(*entry.VLAN).To(Equal(int32(9)))

			_, _, ok = FindReadyIPMetadata(metadata, types.IPVersion(4), newIPs("10.0.0.1", "10.0.0.2", "10.0.0.3"), "node-1", true)
			Expect(ok).To(BeFalse())
		})
	})

	Context("FindGlobalColdPathIP", Labels{"unittest", "FindGlobalColdPathIP"}, func() {
		It("prefers unbound addresses over stealing and skips detaching entries", func() {
			metadata := map[string]spiderpoolv2beta1.IPMetadataEntry{
				"10.0.0.1": {Node: ptr.To("node-2"), VLAN: ptr.To(int32(7))},
				"10.0.0.2": {Node: ptr.To("node-2"), VLAN: ptr.To(int32(-1))},
				"10.0.0.3": {VLAN: ptr.To(int32(-1))},
			}

			// Tier 1: a completed-detach entry (no node) is unbound even with vlan -1.
			ip, ok := FindGlobalColdPathIP(metadata, newIPs("10.0.0.1", "10.0.0.3"))
			Expect(ok).To(BeTrue())
			Expect(ip.String()).To(Equal("10.0.0.3"))

			// Tier 1: an address without any entry beats a bound entry.
			ip, ok = FindGlobalColdPathIP(metadata, newIPs("10.0.0.1", "10.0.0.9"))
			Expect(ok).To(BeTrue())
			Expect(ip.String()).To(Equal("10.0.0.9"))

			// Tier 2: only a bound-on-other-node entry left -> steal it.
			ip, ok = FindGlobalColdPathIP(metadata, newIPs("10.0.0.1"))
			Expect(ok).To(BeTrue())
			Expect(ip.String()).To(Equal("10.0.0.1"))

			// Detaching entries are never candidates.
			_, ok = FindGlobalColdPathIP(metadata, newIPs("10.0.0.2"))
			Expect(ok).To(BeFalse())
		})
	})

	Context("ClassifyColdPath", Labels{"unittest", "ClassifyColdPath"}, func() {
		It("classifies create/rebind/steal from the selected address's entry state", func() {
			metadata := map[string]spiderpoolv2beta1.IPMetadataEntry{
				"10.0.0.1": {Node: ptr.To("node-2"), VLAN: ptr.To(int32(7))},
				"10.0.0.3": {VLAN: ptr.To(int32(-1))},
			}

			// No entry: the provider must create+attach a new sub-ENI.
			Expect(ClassifyColdPath(metadata, "10.0.0.9")).To(Equal(types.IaaSPathColdCreate))
			// Entry without a node: a detached sub-ENI is re-attached.
			Expect(ClassifyColdPath(metadata, "10.0.0.3")).To(Equal(types.IaaSPathColdRebind))
			// Entry bound to another node: idle sub-ENI steal.
			Expect(ClassifyColdPath(metadata, "10.0.0.1")).To(Equal(types.IaaSPathColdSteal))
		})
	})

	Context("FindGlobalColdPathIPv6", Labels{"unittest", "FindGlobalColdPathIPv6"}, func() {
		It("excludes v6 addresses referenced by any sticky pair", func() {
			metadata := map[string]spiderpoolv2beta1.IPMetadataEntry{
				"10.0.0.1": {IPv6: ptr.To("fd00::1"), Node: ptr.To("node-2"), VLAN: ptr.To(int32(7))},
				"10.0.0.2": {IPv6: ptr.To("fd00::2")},
			}
			referenced := MetadataReferencedIPv6Set(metadata)
			Expect(referenced).To(HaveLen(2))

			ip, ok := FindGlobalColdPathIPv6(referenced, newIPs("fd00::1", "fd00::2", "fd00::3"))
			Expect(ok).To(BeTrue())
			Expect(ip.String()).To(Equal("fd00::3"))

			_, ok = FindGlobalColdPathIPv6(referenced, newIPs("fd00::1", "fd00::2"))
			Expect(ok).To(BeFalse())
		})
	})
})
