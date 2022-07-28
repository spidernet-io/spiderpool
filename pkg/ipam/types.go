// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"fmt"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ToBeAllocated struct {
	IPVersion        types.IPVersion
	NIC              string
	DefaultRouteType types.DefaultRouteType
	V4PoolCandidates []string
	V6PoolCandidates []string
}

func (t *ToBeAllocated) String() string {
	return fmt.Sprintf("%+v", *t)
}

type AllocationResult struct {
	IP     *models.IPConfig
	Routes []*models.Route
}
