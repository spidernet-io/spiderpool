// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// fakeRollbackPoolManager extends fakePairPoolManager with a recording
// ReleaseIP so rollbackGlobalPoolClaims can be exercised in isolation.
type fakeRollbackPoolManager struct {
	fakePairPoolManager
	released   map[string][]types.IPAndUID
	releaseErr error
}

func (f *fakeRollbackPoolManager) ReleaseIP(_ context.Context, poolName string, ipAndUIDs []types.IPAndUID) error {
	if f.releaseErr != nil {
		return f.releaseErr
	}
	f.released[poolName] = append(f.released[poolName], ipAndUIDs...)
	return nil
}

var _ = Describe("Global pool cold-path rollback on IaaS allocate failure", Label("ipam_global_rollback_test"), func() {
	var poolManager *fakeRollbackPoolManager
	var i *ipam
	var pod *corev1.Pod

	newResult := func(pool, addr string, fromMetadata bool) *types.AllocationResult {
		return &types.AllocationResult{
			IP: &models.IPConfig{
				IPPool:  pool,
				Address: ptr.To(addr),
				Nic:     ptr.To("eth0"),
			},
			IaaSPath: iaasPathFor(fromMetadata),
		}
	}

	BeforeEach(func() {
		globalPool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "global-pool",
				Labels: map[string]string{
					constant.LabelIPPoolIaasProvider: "huaweicloud",
					constant.LabelIPPoolIaasGlobal:   "true",
				},
			},
		}
		globalV6Pool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "global-pool-v6",
				Labels: map[string]string{
					constant.LabelIPPoolIaasProvider: "huaweicloud",
					constant.LabelIPPoolIaasGlobal:   "true",
				},
			},
		}
		nodePool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-pool",
				Labels: map[string]string{constant.LabelIPPoolIaasProvider: "huaweicloud"},
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{NodeName: []string{"node-1"}},
		}
		staticPool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "static-pool"},
		}
		poolManager = &fakeRollbackPoolManager{
			fakePairPoolManager: fakePairPoolManager{
				pools: map[string]*spiderpoolv2beta1.SpiderIPPool{
					"global-pool":    globalPool,
					"global-pool-v6": globalV6Pool,
					"node-pool":      nodePool,
					"static-pool":    staticPool,
				},
			},
			released: map[string][]types.IPAndUID{},
		}
		i = &ipam{ipPoolManager: poolManager}
		pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", UID: "pod-uid"}}
	})

	It("releases only global-pool claims and keeps the rest for the failure cache", func() {
		results := []*types.AllocationResult{
			newResult("global-pool", "172.18.70.11/24", false),
			newResult("global-pool-v6", "fd00:70::10/120", false),
			newResult("node-pool", "172.18.50.10/24", false),
			newResult("static-pool", "172.18.40.10/24", false),
		}

		kept := i.rollbackGlobalPoolClaims(context.TODO(), pod, results)

		// Both families of the global pair are rolled back with bare IPs.
		Expect(poolManager.released["global-pool"]).To(Equal([]types.IPAndUID{{IP: "172.18.70.11", UID: "pod-uid"}}))
		Expect(poolManager.released["global-pool-v6"]).To(Equal([]types.IPAndUID{{IP: "fd00:70::10", UID: "pod-uid"}}))
		Expect(poolManager.released).To(HaveLen(2))

		// Node-level IaaS and static pool claims stay retry-cacheable.
		Expect(kept).To(HaveLen(2))
		Expect(kept[0].IP.IPPool).To(Equal("node-pool"))
		Expect(kept[1].IP.IPPool).To(Equal("static-pool"))
	})

	It("drops a global-pool claim from the kept set even when its release fails, leaving it to GC", func() {
		poolManager.releaseErr = fmt.Errorf("conflict")
		results := []*types.AllocationResult{
			newResult("global-pool", "172.18.70.11/24", false),
			newResult("static-pool", "172.18.40.10/24", false),
		}

		kept := i.rollbackGlobalPoolClaims(context.TODO(), pod, results)

		Expect(kept).To(HaveLen(1))
		Expect(kept[0].IP.IPPool).To(Equal("static-pool"))
	})

	It("keeps results whose pool cannot be resolved", func() {
		results := []*types.AllocationResult{
			newResult("unknown-pool", "172.18.90.10/24", false),
		}

		kept := i.rollbackGlobalPoolClaims(context.TODO(), pod, results)
		Expect(kept).To(HaveLen(1))
		Expect(poolManager.released).To(BeEmpty())
	})
})
