// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("IPPoolWebhook pair-pool validation", Label("ippool_validate_test"), func() {
	var ctx context.Context
	var count uint64
	var v4PoolName, v6PoolName string
	var v4PoolT, v6PoolT *spiderpoolv2beta1.SpiderIPPool

	newPool := func(name string, ipVersion int64, subnet string, ips []string) *spiderpoolv2beta1.SpiderIPPool {
		return &spiderpoolv2beta1.SpiderIPPool{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.KindSpiderIPPool,
				APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: ptr.To(ipVersion),
				Subnet:    subnet,
				IPs:       ips,
			},
		}
	}

	BeforeEach(func() {
		ippoolmanager.WebhookLogger = logutils.Logger.Named("IPPool-Webhook")
		ipPoolWebhook.EnableIPv4 = true
		ipPoolWebhook.EnableIPv6 = true
		ipPoolWebhook.EnableSpiderSubnet = false

		ctx = context.TODO()

		atomic.AddUint64(&count, 1)
		v4PoolName = fmt.Sprintf("v4-pair-pool-%v", count)
		v6PoolName = fmt.Sprintf("v6-pair-pool-%v", count)

		v4PoolT = newPool(v4PoolName, constant.IPv4, "172.101.40.0/24", []string{"172.101.40.2-172.101.40.3"})
		v6PoolT = newPool(v6PoolName, constant.IPv6, "abcd:1234::/120", []string{"abcd:1234::2-abcd:1234::4"})
	})

	AfterEach(func() {
		for _, pool := range []*spiderpoolv2beta1.SpiderIPPool{v4PoolT, v6PoolT} {
			err := tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderippools",
				},
				pool.Namespace,
				pool.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		}
	})

	It("is unaffected when pair-pool annotation is absent", func() {
		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a self-referential pair-pool", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: v4PoolName,
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).To(HaveOccurred())
	})

	It("allows a reference to a not-yet-existing pool", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: "does-not-exist",
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects pairing pools of the same IP version", func() {
		otherV4PoolName := fmt.Sprintf("other-v4-pool-%v", count)
		otherV4PoolT := newPool(otherV4PoolName, constant.IPv4, "172.102.40.0/24", []string{"172.102.40.2-172.102.40.3"})
		Expect(tracker.Add(otherV4PoolT)).To(Succeed())
		defer func() {
			_ = tracker.Delete(
				schema.GroupVersionResource{Group: constant.SpiderpoolAPIGroup, Version: constant.SpiderpoolAPIVersion, Resource: "spiderippools"},
				otherV4PoolT.Namespace, otherV4PoolT.Name,
			)
		}()

		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: otherV4PoolName,
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).To(HaveOccurred())
	})

	It("rejects when the v4 pool's static capacity exceeds the v6 pool's", func() {
		// v4 pool has 3 static IPs, v6 pool has only 1: v4 > v6, must be rejected.
		v4PoolT.Spec.IPs = []string{"172.101.40.2-172.101.40.4"}
		v6PoolT.Spec.IPs = []string{"abcd:1234::2"}
		Expect(tracker.Add(v6PoolT)).To(Succeed())

		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: v6PoolName,
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).To(HaveOccurred())
	})

	It("allows when the v4 pool's static capacity is <= the v6 pool's", func() {
		Expect(tracker.Add(v6PoolT)).To(Succeed())

		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: v6PoolName,
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects mismatched nodeName between existing paired pools", func() {
		v6PoolT.Spec.NodeName = []string{"node1"}
		Expect(tracker.Add(v6PoolT)).To(Succeed())

		v4PoolT.Spec.NodeName = []string{"node2"}
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: v6PoolName,
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).To(HaveOccurred())
	})

	It("allows matching nodeName/podAffinity between existing paired pools", func() {
		v6PoolT.Spec.NodeName = []string{"node1"}
		Expect(tracker.Add(v6PoolT)).To(Succeed())

		v4PoolT.Spec.NodeName = []string{"node1"}
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolPairPool: v6PoolName,
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows any iaas-provider vendor value", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows an arbitrary iaas-provider vendor value", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "unknown-vendor",
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows an empty iaas-provider vendor value", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "",
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows an arbitrary iaas-provider vendor value on update", func() {
		oldPool := v4PoolT.DeepCopy()
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "unknown-vendor",
		}

		_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects adding spec.nodeName to an IaaS pool on update", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
			constant.AnnoIPPoolParentNic:    "eth1",
		}
		oldPool := v4PoolT.DeepCopy()
		v4PoolT.Spec.NodeName = []string{"node-1"}

		_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, v4PoolT)
		Expect(err).To(HaveOccurred())
	})

	It("rejects removing spec.nodeName from an IaaS pool on update", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
			constant.AnnoIPPoolParentNic:    "eth1",
		}
		v4PoolT.Spec.NodeName = []string{"node-1"}
		oldPool := v4PoolT.DeepCopy()
		v4PoolT.Spec.NodeName = nil

		_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, v4PoolT)
		Expect(err).To(HaveOccurred())
	})

	It("allows changing spec.nodeName between non-empty values on an IaaS pool", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
			constant.AnnoIPPoolParentNic:    "eth1",
		}
		v4PoolT.Spec.NodeName = []string{"node-1"}
		oldPool := v4PoolT.DeepCopy()
		v4PoolT.Spec.NodeName = []string{"node-2"}

		_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows adding or removing spec.nodeName on a non-IaaS pool", func() {
		oldPool := v4PoolT.DeepCopy()
		v4PoolT.Spec.NodeName = []string{"node-1"}

		_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a node-scoped IaaS pool without the parent-nic annotation", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
		}
		v4PoolT.Spec.NodeName = []string{"node-1"}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("node-scoped IaaS pool requires annotation"))
	})

	It("allows a node-scoped IaaS pool with a single parent-nic name", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
			constant.AnnoIPPoolParentNic:    "eth1",
		}
		v4PoolT.Spec.NodeName = []string{"node-1"}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a parent-nic annotation carrying multiple names", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolParentNic: "eth1,eth2",
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must be a single NIC name"))
	})

	It("rejects a parent-nic annotation containing whitespace", func() {
		for _, bad := range []string{"eth1 eth2", "eth1\teth2", "   ", ""} {
			v4PoolT.Annotations = map[string]string{
				constant.AnnoIPPoolParentNic: bad,
			}

			_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
			Expect(err).To(HaveOccurred(), "value %q should be rejected", bad)
		}
	})

	It("allows a global IaaS pool without the parent-nic annotation", func() {
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
		}

		_, err := ipPoolWebhook.ValidateCreate(ctx, v4PoolT)
		Expect(err).NotTo(HaveOccurred())
	})

	It("enforces the node-scoped parent-nic requirement on update too", func() {
		oldPool := v4PoolT.DeepCopy()
		v4PoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: "huaweicloud",
		}
		v4PoolT.Spec.NodeName = []string{"node-1"}

		_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, v4PoolT)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("node-scoped IaaS pool requires annotation"))
	})

	Context("IaaS annotations immutability with allocated IPs", func() {
		var oldPool *spiderpoolv2beta1.SpiderIPPool

		BeforeEach(func() {
			v4PoolT.Annotations = map[string]string{
				constant.AnnoIPPoolIaasProvider: "huaweicloud",
				constant.AnnoIPPoolParentNic:    "eth1",
			}
			v4PoolT.Status.AllocatedIPCount = ptr.To(int64(1))
			oldPool = v4PoolT.DeepCopy()
		})

		It("rejects removing any IaaS annotation while IPs are allocated", func() {
			for _, key := range []string{constant.AnnoIPPoolIaasProvider, constant.AnnoIPPoolParentNic} {
				newPool := oldPool.DeepCopy()
				delete(newPool.Annotations, key)

				_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, newPool)
				Expect(err).To(HaveOccurred(), "removing %s should be rejected", key)
				Expect(err.Error()).To(ContainSubstring("cannot remove annotation"))
			}
		})

		It("rejects modifying the parent-nic annotation while IPs are allocated", func() {
			newPool := oldPool.DeepCopy()
			newPool.Annotations[constant.AnnoIPPoolParentNic] = "eth2"

			_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, newPool)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot modify annotation"))
		})

		It("allows an unrelated update that keeps the IaaS annotations intact", func() {
			newPool := oldPool.DeepCopy()
			newPool.Annotations["other-anno"] = "value"

			_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, newPool)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows removing or modifying the IaaS annotations when no IPs are allocated", func() {
			oldPool.Status.AllocatedIPCount = ptr.To(int64(0))
			newPool := oldPool.DeepCopy()
			newPool.Annotations[constant.AnnoIPPoolParentNic] = "eth2"

			_, err := ipPoolWebhook.ValidateUpdate(ctx, oldPool, newPool)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
