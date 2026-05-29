// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNetworking(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Networking Suite", Label("networking", "unittest"))
}
