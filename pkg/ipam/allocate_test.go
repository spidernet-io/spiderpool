// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("FR-015: IaaS prewarm metadata allocation gating", Label("ipam_allocate_fr015_test"), func() {
	newResult := func(address string, fromMetadata bool) *types.AllocationResult {
		return &types.AllocationResult{
			IP:             &models.IPConfig{Address: ptr.To(address)},
			FromIPMetadata: fromMetadata,
		}
	}

	It("keeps only non-prewarmed results, unaffected when none are metadata-sourced", func() {
		results := []*types.AllocationResult{
			newResult("172.18.70.10/24", false),
			newResult("172.18.70.11/24", false),
		}

		nonPrewarmed := filterNonPrewarmedResults(results)
		Expect(nonPrewarmed).To(HaveLen(2))
		Expect(nonPrewarmed).To(Equal(results))
	})

	It("filters out metadata-sourced results from a mixed batch", func() {
		prewarmedRes := newResult("172.18.70.20/24", true)
		nonPrewarmedRes := newResult("172.18.70.21/24", false)
		results := []*types.AllocationResult{prewarmedRes, nonPrewarmedRes}

		nonPrewarmed := filterNonPrewarmedResults(results)
		Expect(nonPrewarmed).To(Equal([]*types.AllocationResult{nonPrewarmedRes}))
	})

	It("returns an empty (non-nil) slice when every result is metadata-sourced", func() {
		results := []*types.AllocationResult{
			newResult("172.18.70.30/24", true),
			newResult("172.18.70.31/24", true),
		}

		nonPrewarmed := filterNonPrewarmedResults(results)
		Expect(nonPrewarmed).NotTo(BeNil())
		Expect(nonPrewarmed).To(BeEmpty())
	})

	It("handles an empty input without panicking", func() {
		nonPrewarmed := filterNonPrewarmedResults(nil)
		Expect(nonPrewarmed).NotTo(BeNil())
		Expect(nonPrewarmed).To(BeEmpty())
	})
})
