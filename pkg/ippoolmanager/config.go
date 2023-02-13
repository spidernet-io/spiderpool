// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import "time"

const (
	defaultMaxAllocatedIPs = 5000
)

type IPPoolManagerConfig struct {
	MaxConflictRetries    int
	ConflictRetryUnitTime time.Duration
	MaxAllocatedIPs       *int
}

func setDefaultsForIPPoolManagerConfig(config IPPoolManagerConfig) IPPoolManagerConfig {
	if config.MaxAllocatedIPs == nil {
		maxAllocatedIPs := defaultMaxAllocatedIPs
		config.MaxAllocatedIPs = &maxAllocatedIPs
	}

	return config
}
