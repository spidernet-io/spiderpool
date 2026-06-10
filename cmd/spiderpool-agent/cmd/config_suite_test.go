// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Spiderpool Agent Config Suite", Label("spiderpool-agent-config", "unittest"))
}
