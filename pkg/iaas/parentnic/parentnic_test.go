// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package parentnic

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("ParentNic status writer", Label("parentnic_status_writer_test"), func() {
	const nodeName = "node1"

	newScheme := func() *runtime.Scheme {
		scheme := runtime.NewScheme()
		Expect(spiderpoolv2beta1.AddToScheme(scheme)).To(Succeed())
		return scheme
	}

	newNodePool := func(name string, mutate func(*spiderpoolv2beta1.SpiderIPPool)) *spiderpoolv2beta1.SpiderIPPool {
		pool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					constant.LabelIPPoolIaasProvider: "somecloud",
				},
				Annotations: map[string]string{
					constant.AnnoIPPoolIaasProvider: "somecloud",
					constant.AnnoIPPoolParentNic:    "eth1",
				},
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: ptr.To(constant.IPv4),
				Subnet:    "172.18.40.0/24",
				NodeName:  []string{nodeName},
			},
		}
		if mutate != nil {
			mutate(pool)
		}
		return pool
	}

	reconcile := func(pool *spiderpoolv2beta1.SpiderIPPool, resolver macResolver) (*spiderpoolv2beta1.SpiderIPPool, error) {
		cli := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(pool).
			WithStatusSubresource(pool).
			Build()
		w := &StatusWriter{
			client:     cli,
			nodeName:   nodeName,
			resolveMac: resolver,
			logger:     logutils.Logger,
		}
		_, err := w.Reconcile(context.TODO(),
			ctrl.Request{NamespacedName: apitypes.NamespacedName{Name: pool.Name}})
		got := &spiderpoolv2beta1.SpiderIPPool{}
		Expect(cli.Get(context.TODO(), apitypes.NamespacedName{Name: pool.Name}, got)).To(Succeed())
		return got, err
	}

	staticResolver := func(mac string) macResolver {
		return func(nicName string) (string, error) {
			Expect(nicName).To(Equal("eth1"))
			return mac, nil
		}
	}

	It("publishes name and MAC to status.parentNic of an eligible node-level pool", func() {
		got, err := reconcile(newNodePool("pool-v4", nil), staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).To(Equal(&spiderpoolv2beta1.ParentNicStatus{
			Name: "eth1",
			MAC:  "fa:16:3e:aa:bb:cc",
		}))
	})

	It("is idempotent when status.parentNic is already up to date", func() {
		pool := newNodePool("pool-v4", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Status.ParentNic = &spiderpoolv2beta1.ParentNicStatus{Name: "eth1", MAC: "fa:16:3e:aa:bb:cc"}
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.ResourceVersion).To(Equal(pool.ResourceVersion))
	})

	It("refreshes a stale MAC", func() {
		pool := newNodePool("pool-v4", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Status.ParentNic = &spiderpoolv2beta1.ParentNicStatus{Name: "eth1", MAC: "fa:16:3e:00:00:00"}
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic.MAC).To(Equal("fa:16:3e:aa:bb:cc"))
	})

	It("returns an error for retry when MAC resolution fails", func() {
		_, err := reconcile(newNodePool("pool-v4", nil), func(string) (string, error) {
			return "", fmt.Errorf("no such link")
		})
		Expect(err).To(HaveOccurred())
	})

	It("skips a non-IaaS pool", func() {
		pool := newNodePool("plain", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Labels = nil
			p.Annotations = nil
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).To(BeNil())
	})

	It("skips a pool pinned to another node", func() {
		pool := newNodePool("other-node", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Spec.NodeName = []string{"node2"}
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).To(BeNil())
	})

	It("skips a global pool (no spec.nodeName)", func() {
		pool := newNodePool("global", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Spec.NodeName = nil
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).To(BeNil())
	})

	It("skips the sibling v6 pool of a paired set but serves a single-stack v6 pool", func() {
		sibling := newNodePool("pool-v6-sibling", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Spec.IPVersion = ptr.To(constant.IPv6)
			p.Spec.Subnet = "fd00:172:18::/64"
			p.Annotations[constant.AnnoIPPoolPairPool] = "pool-v4"
		})
		got, err := reconcile(sibling, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).To(BeNil())

		single := newNodePool("pool-v6-single", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Spec.IPVersion = ptr.To(constant.IPv6)
			p.Spec.Subnet = "fd00:172:18::/64"
		})
		got, err = reconcile(single, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).NotTo(BeNil())
	})

	It("skips a node-level pool without the parent-nic annotation", func() {
		pool := newNodePool("legacy", func(p *spiderpoolv2beta1.SpiderIPPool) {
			delete(p.Annotations, constant.AnnoIPPoolParentNic)
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).To(BeNil())
	})

	It("does not stomp provider-owned ipMetaData when patching", func() {
		pool := newNodePool("pool-v4", func(p *spiderpoolv2beta1.SpiderIPPool) {
			p.Status.IPMetaData = &spiderpoolv2beta1.IPMetaData{
				Metadata:           ptr.To(`{"scope":"node1","ips":{}}`),
				ObservedGeneration: ptr.To(int64(1)),
			}
		})
		got, err := reconcile(pool, staticResolver("fa:16:3e:aa:bb:cc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Status.ParentNic).NotTo(BeNil())
		Expect(got.Status.IPMetaData).NotTo(BeNil())
		Expect(*got.Status.IPMetaData.Metadata).To(Equal(`{"scope":"node1","ips":{}}`))
	})
})

var _ = Describe("ResolveLocalNicMac", Label("parentnic_status_writer_test"), func() {
	It("fails for a non-existent link", func() {
		_, err := ResolveLocalNicMac("no-such-nic-xyz")
		Expect(err).To(HaveOccurred())
	})

	It("resolves the MAC of the loopback-adjacent existing NIC when available", func() {
		// "lo" exists everywhere but has an all-zero MAC that formats as
		// empty; assert only that the function does not panic and returns
		// either a MAC or a clear error.
		mac, err := ResolveLocalNicMac("lo")
		if err == nil {
			Expect(mac).NotTo(BeEmpty())
		}
	})
})
