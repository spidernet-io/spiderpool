// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package config

import "time"

type UpdateCRConfig struct {
	MaxConflictRetries    int
	ConflictRetryUnitTime time.Duration
}
