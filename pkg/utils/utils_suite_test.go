// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package utils

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite", Label("utils", "unittest"))
}
