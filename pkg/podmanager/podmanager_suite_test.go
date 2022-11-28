// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPodmanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Podmanager Suite")
}
