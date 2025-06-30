// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package nri

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNri(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NRI Suite")
}
