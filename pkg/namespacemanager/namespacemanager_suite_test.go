// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNamespacemanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Namespacemanager Suite")
}
