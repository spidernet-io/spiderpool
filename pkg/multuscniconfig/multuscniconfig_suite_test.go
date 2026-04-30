// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMultusCNIConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MultusCNIConfig Suite")
}
