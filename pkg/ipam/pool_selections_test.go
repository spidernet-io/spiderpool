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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// fakePairPoolManager is a minimal ippoolmanager.IPPoolManager implementation
// exercising only GetIPPoolByName/ParseWildcardPoolNameList, which are all
// that getPoolFromPodAnnoPool/completeWithPairPools require for this test.
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

func (f *fakePairPoolManager) AllocateIP(context.Context, string, string, *corev1.Pod, types.PodTopController) (*models.IPConfig, bool, error) {
	return nil, false, fmt.Errorf("not implemented")
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

var _ = Describe("Automatic dual-stack pool completion", Label("ipam_pool_selections_test"), func() {
	var v4Pool, v6Pool, unpairedPool *spiderpoolv2beta1.SpiderIPPool

	newPool := func(name, pairName string) *spiderpoolv2beta1.SpiderIPPool {
		p := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}
		if pairName != "" {
			p.Annotations = map[string]string{constant.AnnoIPPoolPairPool: pairName}
		}
		return p
	}

	BeforeEach(func() {
		v4Pool = newPool("v4-pool", "v6-pool")
		v6Pool = newPool("v6-pool", "v4-pool")
		unpairedPool = newPool("unpaired-pool", "")
	})

	It("auto-completes a single-family v4 request with a valid pair-pool", func() {
		inst := &ipam{
			config: IPAMConfig{EnableIPv4: true, EnableIPv6: true},
			ipPoolManager: &fakePairPoolManager{pools: map[string]*spiderpoolv2beta1.SpiderIPPool{
				v4Pool.Name: v4Pool,
				v6Pool.Name: v6Pool,
			}},
		}

		anno := `{"ipv4":["v4-pool"]}`
		t, err := inst.getPoolFromPodAnnoPool(context.TODO(), anno, "eth0", false)
		Expect(err).NotTo(HaveOccurred())

		Expect(t.PoolCandidates).To(HaveLen(2))
		var v4Names, v6Names []string
		for _, c := range t.PoolCandidates {
			switch c.IPVersion {
			case constant.IPv4:
				v4Names = c.Pools
			case constant.IPv6:
				v6Names = c.Pools
			}
		}
		Expect(v4Names).To(ConsistOf("v4-pool"))
		Expect(v6Names).To(ConsistOf("v6-pool"))
	})

	It("does not duplicate an already-explicit dual-stack request", func() {
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

	It("leaves a pool without pair-pool unaffected", func() {
		inst := &ipam{
			config: IPAMConfig{EnableIPv4: true, EnableIPv6: true},
			ipPoolManager: &fakePairPoolManager{pools: map[string]*spiderpoolv2beta1.SpiderIPPool{
				unpairedPool.Name: unpairedPool,
			}},
		}

		anno := `{"ipv4":["unpaired-pool"]}`
		t, err := inst.getPoolFromPodAnnoPool(context.TODO(), anno, "eth0", false)
		Expect(err).NotTo(HaveOccurred())

		Expect(t.PoolCandidates).To(HaveLen(1))
		Expect(t.PoolCandidates[0].IPVersion).To(Equal(constant.IPv4))
		Expect(t.PoolCandidates[0].Pools).To(ConsistOf("unpaired-pool"))
	})

	It("auto-completes a wildcard-expanded pool that carries pair-pool", func() {
		inst := &ipam{
			config: IPAMConfig{EnableIPv4: true, EnableIPv6: true},
			ipPoolManager: &fakePairPoolManager{pools: map[string]*spiderpoolv2beta1.SpiderIPPool{
				v4Pool.Name: v4Pool,
				v6Pool.Name: v6Pool,
			}},
		}

		// Simulate a wildcard already resolved to a concrete pool name by
		// ParseWildcardPoolNameList (the fake here is a passthrough, mirroring
		// what happens once the wildcard is resolved upstream).
		anno := `{"ipv4":["v4-pool"]}`
		t, err := inst.getPoolFromPodAnnoPool(context.TODO(), anno, "eth0", false)
		Expect(err).NotTo(HaveOccurred())

		var v6Names []string
		for _, c := range t.PoolCandidates {
			if c.IPVersion == constant.IPv6 {
				v6Names = c.Pools
			}
		}
		Expect(v6Names).To(ConsistOf("v6-pool"))
	})
})
