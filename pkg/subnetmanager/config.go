// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"time"

	"github.com/spidernet-io/spiderpool/pkg/config"
)

type SubnetManagerConfig struct {
	EnableIPv4 bool
	EnableIPv6 bool

	config.UpdateCRConfig
	EnableSpiderSubnet            bool
	EnableSubnetDeleteStaleIPPool bool
	LeaderRetryElectGap           time.Duration
	ResyncPeriod                  time.Duration

	Workers            int
	MaxWorkqueueLength int
}
