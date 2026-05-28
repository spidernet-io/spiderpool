// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNetworkingIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Networking Integration Suite", Label("networking_integration", "unittest"))
}
