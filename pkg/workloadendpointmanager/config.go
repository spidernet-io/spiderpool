// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"time"
)

type EndpointManagerConfig struct {
	MaxConflictRetries    int
	ConflictRetryUnitTime time.Duration
}

func setDefaultsForEndpointManagerConfig(config EndpointManagerConfig) EndpointManagerConfig {
	return config
}
