// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import "github.com/spidernet-io/spiderpool/pkg/config"

type EndpointManagerConfig struct {
	config.UpdateCRConfig
	MaxHistoryRecords int
}
