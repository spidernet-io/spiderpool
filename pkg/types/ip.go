// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import "github.com/spidernet-io/spiderpool/api/v1/agent/models"

type IPVersion = int64

type Vlan = int64

type AllocationResult struct {
	IP           *models.IPConfig
	Routes       []*models.Route
	CleanGateway bool

	// FromIaasLedger is true when IP was selected from a pool's
	// prewarm ledger (status.iaasIPs), meaning it is already known to be
	// ready for use and MUST NOT trigger the synchronous IaaS provider
	// allocation call (see callIaaSAllocate in pkg/ipam/iaas.go).
	FromIaasLedger bool
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
