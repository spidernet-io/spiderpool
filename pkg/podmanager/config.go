// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"time"
)

type PodManagerConfig struct {
	MaxConflictRetries    int
	ConflictRetryUnitTime time.Duration
}

func setDefaultsForPodManagerConfig(config PodManagerConfig) PodManagerConfig {
	return config
}
