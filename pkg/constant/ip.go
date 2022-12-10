// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import "github.com/spidernet-io/spiderpool/pkg/types"

const (
	IPv4 types.IPVersion = 4
	IPv6 types.IPVersion = 6
)

const (
	InvalidIPVersion = types.IPVersion(976)
	InvalidCIDR      = "invalid CIDR"
	InvalidIP        = "invalid IP"
	InvalidIPRange   = "invalid IP range"
	InvalidDst       = "invalid routing destination"
	InvalidGateway   = "invalid routing gateway"
)

var InvalidIPRanges = []string{InvalidIPRange}
