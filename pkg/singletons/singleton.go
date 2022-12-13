// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package singletons

import "github.com/spidernet-io/spiderpool/pkg/types"

// ClusterDefaultPool is a singleton recording cluster default IPPool and Subnet configurations
var ClusterDefaultPool = new(types.ClusterDefaultPoolConfig)

// InitClusterDefaultPool will init ClusterDefaultPool with the given params
func InitClusterDefaultPool(clusterDefaultV4IPPool, clusterDefaultV6IPPool, clusterDefaultV4Subnet, clusterDefaultV6Subnet []string, flexibleIPNumber int) {
	ClusterDefaultPool.ClusterDefaultIPv4IPPool = clusterDefaultV4IPPool
	ClusterDefaultPool.ClusterDefaultIPv6IPPool = clusterDefaultV6IPPool
	ClusterDefaultPool.ClusterDefaultIPv4Subnet = clusterDefaultV4Subnet
	ClusterDefaultPool.ClusterDefaultIPv6Subnet = clusterDefaultV6Subnet
	ClusterDefaultPool.ClusterDefaultSubnetFlexibleIPNumber = flexibleIPNumber
}
