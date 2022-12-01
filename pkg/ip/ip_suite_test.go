// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/types"
)

const (
	invalidIPVersion = types.IPVersion(976)
	invalidCIDR      = "invalid CIDR"
	invalidIP        = "invalid IP"
	invalidIPRange   = "invalid IP range"
	invalidDst       = "invalid routing destination"
	invalidGateway   = "invalid routing gateway"
)

var invalidIPRanges = []string{invalidIPRange}

func TestIP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IP Suite", Label("ip", "unitest"))
}
