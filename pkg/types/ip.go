// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import "github.com/spidernet-io/spiderpool/api/v1/agent/models"

type IPVersion = int64

type Vlan = int64

// IaaSAllocationPath classifies how an address was selected from an IaaS
// pool, from the consumer's selection-time point of view. The provider's
// Allocate RPC response remains authoritative for the actual cloud actions;
// this classification is a best-effort prediction used for metrics.
type IaaSAllocationPath string

const (
	// IaaSPathCacheHit is the zero-RPC hot path: a ready prewarmed entry
	// (node pool) or a locally-bound sub-ENI (global pool) is reused
	// directly without any provider RPC.
	IaaSPathCacheHit IaaSAllocationPath = "cache_hit"
	// IaaSPathColdCreate is the cold path with no metadata entry for the
	// address: the provider must create and attach a new sub-ENI
	// (one cloud call).
	IaaSPathColdCreate IaaSAllocationPath = "cold_create"
	// IaaSPathColdRebind is the cold path where a sub-ENI already exists
	// but is detached: the provider only re-attaches it (one cloud call).
	IaaSPathColdRebind IaaSAllocationPath = "cold_rebind"
	// IaaSPathColdSteal is the cold path where the sub-ENI is idle on
	// another node: the provider must detach it there and attach it locally
	// (two cloud calls).
	IaaSPathColdSteal IaaSAllocationPath = "cold_steal"
)

// IsCacheHit reports whether the path is the zero-RPC prewarm hit, meaning
// the address is already known to be ready for use and MUST NOT trigger the
// synchronous IaaS provider allocation call.
func (p IaaSAllocationPath) IsCacheHit() bool {
	return p == IaaSPathCacheHit
}

type AllocationResult struct {
	IP           *models.IPConfig
	Routes       []*models.Route
	CleanGateway bool

	// IaaSPath classifies how the address was selected from an IaaS pool
	// (empty for non-IaaS pools). A cache-hit path means the IP was
	// selected via the pool's provider-written prewarm metadata
	// (status.ipMetaData.metadata) and MUST NOT trigger the synchronous
	// IaaS provider allocation call (see callIaaSAllocate in
	// pkg/ipam/iaas.go); any cold path requires that call.
	IaaSPath IaaSAllocationPath
}

type IPAndUID struct {
	IP  string
	UID string
}

type PoolNameToIPAndUIDs map[string][]IPAndUID

func (pius *PoolNameToIPAndUIDs) Pools() []string {
	var pools []string
	for pool := range *pius {
		pools = append(pools, pool)
	}

	return pools
}
