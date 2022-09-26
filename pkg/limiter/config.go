// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package limiter

import "time"

type LimiterConfig struct {
	MaxQueueSize int
	MaxWaitTime  time.Duration
}
