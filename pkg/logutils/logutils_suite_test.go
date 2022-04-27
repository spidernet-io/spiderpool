// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package logutils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLogutils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logutils Suite")
}
