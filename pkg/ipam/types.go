// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"fmt"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ToBeAllocated struct {
	NIC            string
	CleanGateway   bool
	PoolCandidates []*PoolCandidate
}

type PoolCandidate struct {
	IPVersion types.IPVersion
	Pools     []string
}

func (t *ToBeAllocated) String() string {
	return fmt.Sprintf("%+v", *t)
}

func (c *PoolCandidate) String() string {
	return fmt.Sprintf("%+v", *c)
}

type AllocationResult struct {
	IP           *models.IPConfig
	Routes       []*models.Route
	CleanGateway bool
}
