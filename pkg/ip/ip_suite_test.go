// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IP Suite", Label("ip", "unittest"))
}
