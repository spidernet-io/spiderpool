// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIPAM(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IPAM Suite", Label("ipam", "unittest"))
}
