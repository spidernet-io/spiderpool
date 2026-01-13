// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"fmt"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ToBeAllocateds []*ToBeAllocated

func (tt *ToBeAllocateds) Pools() []string {
	var pools []string
	for _, t := range *tt {
		pools = append(pools, t.Pools()...)
	}

	return pools
}

func (tt *ToBeAllocateds) Candidates() []*PoolCandidate {
	var candidates []*PoolCandidate
	for _, t := range *tt {
		candidates = append(candidates, t.PoolCandidates...)
	}

	return candidates
}

type ToBeAllocated struct {
	NIC            string
	CleanGateway   bool
	PoolCandidates []*PoolCandidate
}

func (t *ToBeAllocated) Pools() []string {
	var pools []string
	for _, c := range t.PoolCandidates {
		pools = append(pools, c.Pools...)
	}

	return pools
}

func (t *ToBeAllocated) String() string {
	return fmt.Sprintf("%+v", *t)
}

type PoolCandidate struct {
	IPVersion types.IPVersion
	Pools     []string
	PToIPPool PoolNameToIPPool
}

func (c *PoolCandidate) String() string {
	return fmt.Sprintf("%+v", *c)
}

type PoolNameToIPPool map[string]*spiderpoolv2beta1.SpiderIPPool

func (pp *PoolNameToIPPool) IPPools() []*spiderpoolv2beta1.SpiderIPPool {
	var ipPools []*spiderpoolv2beta1.SpiderIPPool
	for _, p := range *pp {
		ipPools = append(ipPools, p)
	}

	return ipPools
}

func (pp PoolNameToIPPool) String() string {
	return "*"
}
