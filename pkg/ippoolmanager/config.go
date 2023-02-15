// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"github.com/spidernet-io/spiderpool/pkg/config"
)

type IPPoolManagerConfig struct {
	config.UpdateCRConfig
	MaxAllocatedIPs int
}
