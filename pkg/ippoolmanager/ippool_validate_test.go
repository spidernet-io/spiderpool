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
})
