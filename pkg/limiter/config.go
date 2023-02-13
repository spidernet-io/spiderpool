// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package limiter

const (
	defaultMaxQueueSize = 1000
)

type LimiterConfig struct {
	MaxQueueSize *int
}

func setDefaultsForLimiterConfig(config LimiterConfig) LimiterConfig {
	if config.MaxQueueSize == nil {
		maxQueueSize := defaultMaxQueueSize
		config.MaxQueueSize = &maxQueueSize
	}

	return config
}
