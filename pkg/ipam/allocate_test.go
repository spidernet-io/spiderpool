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

var _ = Describe("FR-015: IaaS prewarm ledger allocation gating", Label("ipam_allocate_fr015_test"), func() {
	newResult := func(address string, fromLedger bool) *types.AllocationResult {
		return &types.AllocationResult{
			IP:             &models.IPConfig{Address: ptr.To(address)},
			FromIaasLedger: fromLedger,
		}
	}

	It("keeps only non-ledger results, unaffected when none are ledger-sourced", func() {
		results := []*types.AllocationResult{
			newResult("172.18.70.10/24", false),
			newResult("172.18.70.11/24", false),
		}

		nonLedger := filterNonLedgerResults(results)
		Expect(nonLedger).To(HaveLen(2))
		Expect(nonLedger).To(Equal(results))
	})

	It("filters out ledger-sourced results from a mixed batch", func() {
		ledgerRes := newResult("172.18.70.20/24", true)
		nonLedgerRes := newResult("172.18.70.21/24", false)
		results := []*types.AllocationResult{ledgerRes, nonLedgerRes}

		nonLedger := filterNonLedgerResults(results)
		Expect(nonLedger).To(Equal([]*types.AllocationResult{nonLedgerRes}))
	})

	It("returns an empty (non-nil) slice when every result is ledger-sourced", func() {
		results := []*types.AllocationResult{
			newResult("172.18.70.30/24", true),
			newResult("172.18.70.31/24", true),
		}

		nonLedger := filterNonLedgerResults(results)
		Expect(nonLedger).NotTo(BeNil())
		Expect(nonLedger).To(BeEmpty())
	})

	It("handles an empty input without panicking", func() {
		nonLedger := filterNonLedgerResults(nil)
		Expect(nonLedger).NotTo(BeNil())
		Expect(nonLedger).To(BeEmpty())
	})
})
