// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"time"

	"github.com/spidernet-io/spiderpool/pkg/config"
)

type IPPoolManagerConfig struct {
	EnableIPv4 bool
	EnableIPv6 bool

	config.UpdateCRConfig
	EnableSpiderSubnet  bool
	MaxAllocatedIPs     int
	LeaderRetryElectGap time.Duration
	MaxWorkQueueLength  int
}
