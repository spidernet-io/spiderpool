// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package config

import "time"

type UpdateCRConfig struct {
	MaxConflictRetrys     int
	ConflictRetryUnitTime time.Duration
}
