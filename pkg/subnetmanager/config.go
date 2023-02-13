// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"time"
)

type SubnetManagerConfig struct {
	MaxConflictRetries    int
	ConflictRetryUnitTime time.Duration
}

func setDefaultsForSubnetManagerConfig(config SubnetManagerConfig) SubnetManagerConfig {
	return config
}
