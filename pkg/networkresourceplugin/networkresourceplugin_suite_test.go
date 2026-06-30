// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNetworkResourcePlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Resource Plugin Suite", Label("networkresourceplugin", "unittest"))
}
