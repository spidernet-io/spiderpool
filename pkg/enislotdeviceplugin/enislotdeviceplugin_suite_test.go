// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestENISlotDevicePlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ENI Slot Device Plugin Suite", Label("enislotdeviceplugin", "unittest"))
}
