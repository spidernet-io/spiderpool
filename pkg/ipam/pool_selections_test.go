// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// fakePairPoolManager is a minimal ippoolmanager.IPPoolManager implementation
// exercising only GetIPPoolByName/ParseWildcardPoolNameList, which are all
// that getPoolFromPodAnnoPool requires for this test.
type fakePairPoolManager struct {
	pools map[string]*spiderpoolv2beta1.SpiderIPPool
}

func (f *fakePairPoolManager) GetIPPoolByName(_ context.Context, poolName string, _ bool) (*spiderpoolv2beta1.SpiderIPPool, error) {
	pool, ok := f.pools[poolName]
	if !ok {
		return nil, apierrors.NewNotFound(spiderpoolv2beta1.Resource(constant.KindSpiderIPPool), poolName)
	}
	return pool, nil
}

func (f *fakePairPoolManager) ListIPPools(context.Context, bool, ...client.ListOption) (*spiderpoolv2beta1.SpiderIPPoolList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakePairPoolManager) AllocateIP(context.Context, string, string, *corev1.Pod, types.PodTopController) (*models.IPConfig, types.IaaSAllocationPath, error) {
	return nil, "", fmt.Errorf("not implemented")
}

func (f *fakePairPoolManager) AllocateIPPair(context.Context, string, string, *corev1.Pod, types.PodTopController) (*models.IPConfig, *models.IPConfig, types.IaaSAllocationPath, error) {
	return nil, nil, "", fmt.Errorf("not implemented")
}

func (f *fakePairPoolManager) ReleaseIP(context.Context, string, []types.IPAndUID) error {
	return fmt.Errorf("not implemented")
}

func (f *fakePairPoolManager) UpdateAllocatedIPs(context.Context, string, string, []types.IPAndUID) error {
	return fmt.Errorf("not implemented")
}

func (f *fakePairPoolManager) ParseWildcardPoolNameList(_ context.Context, poolNames []string, _ types.IPVersion) ([]string, bool, error) {
	return poolNames, false, nil
}

var _ = Describe("Paired IaaS pool candidate selection", Label("ipam_pool_selections_test"), func() {
	newPool := func(name, pairName string) *spiderpoolv2beta1.SpiderIPPool {
		p := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}
		if pairName != "" {
			p.Annotations = map[string]string{constant.AnnoIPPoolPairPool: pairName}
		}
		return p
	}

	It("does not auto-complete the sibling v6 pool of a paired v4 pool into the candidates", func() {
		v4Pool := newPool("v4-pool", "v6-pool")
		v6Pool := newPool("v6-pool", "")
		inst := &ipam{
			config: IPAMConfig{EnableIPv4: true, EnableIPv6: true},
			ipPoolManager: &fakePairPoolManager{pools: map[string]*spiderpoolv2beta1.SpiderIPPool{
				v4Pool.Name: v4Pool,
				v6Pool.Name: v6Pool,
			}},
		}

		// The Pod declares only the paired v4 primary pool. The sibling v6
		// pool must NOT be auto-completed as a v6 candidate: both families
		// are allocated together from the v4 pool's metadata by
		// AllocateIPPair, so the sibling pool never allocates on its own.
		anno := `{"ipv4":["v4-pool"]}`
		t, err := inst.getPoolFromPodAnnoPool(context.TODO(), anno, "eth0", false)
		Expect(err).NotTo(HaveOccurred())

		Expect(t.PoolCandidates).To(HaveLen(1))
		Expect(t.PoolCandidates[0].IPVersion).To(Equal(constant.IPv4))
		Expect(t.PoolCandidates[0].Pools).To(ConsistOf("v4-pool"))
	})

	It("keeps an explicit dual-stack request untouched", func() {
		v4Pool := newPool("v4-pool", "")
		v6Pool := newPool("v6-pool", "")
		inst := &ipam{
			config: IPAMConfig{EnableIPv4: true, EnableIPv6: true},
			ipPoolManager: &fakePairPoolManager{pools: map[string]*spiderpoolv2beta1.SpiderIPPool{
				v4Pool.Name: v4Pool,
				v6Pool.Name: v6Pool,
			}},
		}

		anno := `{"ipv4":["v4-pool"],"ipv6":["v6-pool"]}`
		t, err := inst.getPoolFromPodAnnoPool(context.TODO(), anno, "eth0", false)
		Expect(err).NotTo(HaveOccurred())

		Expect(t.PoolCandidates).To(HaveLen(2))
		for _, c := range t.PoolCandidates {
			Expect(c.Pools).To(HaveLen(1))
		}
	})
})

var _ = Describe("Dropping superseded v6 candidates for paired IaaS pools", Label("ipam_pool_selections_test"), func() {
	newIaaSV4Pool := func(name, pairName string) *spiderpoolv2beta1.SpiderIPPool {
		return &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: map[string]string{constant.AnnoIPPoolPairPool: pairName},
				Labels:      map[string]string{constant.LabelIPPoolIaasProvider: "huaweicloud"},
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{IPVersion: ptr.To(constant.IPv4)},
		}
	}
	newPlainPool := func(name string, version int64) *spiderpoolv2beta1.SpiderIPPool {
		return &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       spiderpoolv2beta1.IPPoolSpec{IPVersion: ptr.To(version)},
		}
	}

	It("ignores explicitly configured v6 pools when the v4 candidate contains a paired IaaS primary pool", func() {
		inst := &ipam{config: IPAMConfig{EnableIPv4: true, EnableIPv6: true}}
		pairedV4 := newIaaSV4Pool("paired-v4", "paired-v6")
		otherV6 := newPlainPool("other-v6", constant.IPv6)
		t := &ToBeAllocated{
			NIC: "eth0",
			PoolCandidates: []*PoolCandidate{
				{IPVersion: constant.IPv4, Pools: []string{"paired-v4"}, PToIPPool: PoolNameToIPPool{"paired-v4": pairedV4}},
				{IPVersion: constant.IPv6, Pools: []string{"other-v6"}, PToIPPool: PoolNameToIPPool{"other-v6": otherV6}},
			},
		}

		inst.dropSupersededV6Candidates(context.TODO(), t)

		// The pair allocation supplies the v6 address; the separately
		// configured v6 pool would otherwise yield a duplicate IPv6.
		Expect(t.PoolCandidates).To(HaveLen(1))
		Expect(t.PoolCandidates[0].IPVersion).To(Equal(constant.IPv4))
	})

	It("keeps v6 candidates when no v4 candidate pool is a paired IaaS primary pool", func() {
		inst := &ipam{config: IPAMConfig{EnableIPv4: true, EnableIPv6: true}}
		plainV4 := newPlainPool("plain-v4", constant.IPv4)
		plainV6 := newPlainPool("plain-v6", constant.IPv6)
		t := &ToBeAllocated{
			NIC: "eth0",
			PoolCandidates: []*PoolCandidate{
				{IPVersion: constant.IPv4, Pools: []string{"plain-v4"}, PToIPPool: PoolNameToIPPool{"plain-v4": plainV4}},
				{IPVersion: constant.IPv6, Pools: []string{"plain-v6"}, PToIPPool: PoolNameToIPPool{"plain-v6": plainV6}},
			},
		}

		inst.dropSupersededV6Candidates(context.TODO(), t)

		Expect(t.PoolCandidates).To(HaveLen(2))
	})

	It("keeps v6 candidates when IPv6 is disabled (single-stack: the paired pool allocates v4 only)", func() {
		inst := &ipam{config: IPAMConfig{EnableIPv4: true, EnableIPv6: false}}
		pairedV4 := newIaaSV4Pool("paired-v4", "paired-v6")
		t := &ToBeAllocated{
			NIC: "eth0",
			PoolCandidates: []*PoolCandidate{
				{IPVersion: constant.IPv4, Pools: []string{"paired-v4"}, PToIPPool: PoolNameToIPPool{"paired-v4": pairedV4}},
			},
		}

		inst.dropSupersededV6Candidates(context.TODO(), t)

		Expect(t.PoolCandidates).To(HaveLen(1))
	})
})

var _ = Describe("IaaS candidate pool class exclusivity", Label("ipam_pool_selections_test"), func() {
	newPool := func(name string, version int64, iaas bool, pairName string) *spiderpoolv2beta1.SpiderIPPool {
		p := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: ptr.To(version),
				Disable:   ptr.To(false),
			},
		}
		if iaas {
			p.Labels = map[string]string{constant.LabelIPPoolIaasProvider: "huaweicloud"}
		}
		if pairName != "" {
			p.Annotations = map[string]string{constant.AnnoIPPoolPairPool: pairName}
		}
		return p
	}
	newInst := func(enableV6 bool) *ipam {
		return &ipam{config: IPAMConfig{
			EnableIPv4:           true,
			EnableIPv6:           enableV6,
			MultusClusterNetwork: ptr.To("kube-system/macvlan"),
			AgentNamespace:       "kube-system",
		}}
	}
	newPod := func() *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "pod"}}
	}
	newCandidate := func(pools ...*spiderpoolv2beta1.SpiderIPPool) *PoolCandidate {
		c := &PoolCandidate{IPVersion: constant.IPv4, PToIPPool: PoolNameToIPPool{}}
		for _, p := range pools {
			c.Pools = append(c.Pools, p.Name)
			c.PToIPPool[p.Name] = p
		}
		return c
	}

	It("keeps only paired IaaS primary pools when the candidate mixes all classes", func() {
		inst := newInst(true)
		paired := newPool("paired-v4", constant.IPv4, true, "paired-v6")
		iaas := newPool("iaas-v4", constant.IPv4, true, "")
		plain := newPool("plain-v4", constant.IPv4, false, "")
		t := &ToBeAllocated{NIC: "eth0", PoolCandidates: []*PoolCandidate{newCandidate(paired, iaas, plain)}}

		Expect(inst.filterPoolCandidates(context.TODO(), t, newPod(), types.PodTopController{}, &models.IpamAddArgs{})).To(Succeed())

		c := t.PoolCandidates[0]
		Expect(c.Pools).To(ConsistOf("paired-v4"))
		Expect(c.PToIPPool).To(HaveLen(1))
	})

	It("keeps only IaaS pools when the candidate mixes IaaS and plain pools", func() {
		inst := newInst(true)
		prewarm := newPool("prewarm-v4", constant.IPv4, true, "")
		prewarm.Spec.NodeName = []string{"node-1"}
		global := newPool("global-v4", constant.IPv4, true, "")
		plain := newPool("plain-v4", constant.IPv4, false, "")
		t := &ToBeAllocated{NIC: "eth0", PoolCandidates: []*PoolCandidate{newCandidate(prewarm, global, plain)}}
		pod := newPod()
		pod.Spec.NodeName = "node-1"

		Expect(inst.filterPoolCandidates(context.TODO(), t, pod, types.PodTopController{}, &models.IpamAddArgs{})).To(Succeed())

		// Node prewarm and global pools coexist (both IaaS class); only the
		// plain pool is dropped.
		Expect(t.PoolCandidates[0].Pools).To(ConsistOf("prewarm-v4", "global-v4"))
	})

	It("keeps a pure plain-pool candidate untouched", func() {
		inst := newInst(true)
		plainA := newPool("plain-a", constant.IPv4, false, "")
		plainB := newPool("plain-b", constant.IPv4, false, "")
		t := &ToBeAllocated{NIC: "eth0", PoolCandidates: []*PoolCandidate{newCandidate(plainA, plainB)}}

		Expect(inst.filterPoolCandidates(context.TODO(), t, newPod(), types.PodTopController{}, &models.IpamAddArgs{})).To(Succeed())

		Expect(t.PoolCandidates[0].Pools).To(ConsistOf("plain-a", "plain-b"))
	})

	It("treats a paired primary pool as an ordinary IaaS pool when IPv6 is disabled", func() {
		inst := newInst(false)
		paired := newPool("paired-v4", constant.IPv4, true, "paired-v6")
		iaas := newPool("iaas-v4", constant.IPv4, true, "")
		plain := newPool("plain-v4", constant.IPv4, false, "")
		t := &ToBeAllocated{NIC: "eth0", PoolCandidates: []*PoolCandidate{newCandidate(paired, iaas, plain)}}

		Expect(inst.filterPoolCandidates(context.TODO(), t, newPod(), types.PodTopController{}, &models.IpamAddArgs{})).To(Succeed())

		// Single-stack: pair allocation does not engage, so both IaaS pools
		// share the same class and only the plain pool is dropped.
		Expect(t.PoolCandidates[0].Pools).To(ConsistOf("paired-v4", "iaas-v4"))
	})
})
