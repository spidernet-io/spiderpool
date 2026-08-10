// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package parentnic

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestParentNic(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ParentNic Suite")
}
