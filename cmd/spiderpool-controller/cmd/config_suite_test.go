// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestControllerConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Spiderpool Controller Config Suite", Label("spiderpool-controller-config", "unittest"))
}
