// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var _ = Describe("ENI slot devices", Label("enislotdeviceplugin_devices_test"), func() {
	It("generates stable healthy slot IDs from maxSlotsPerNode", func() {
		devices := healthyDevices(3)

		Expect(devices).To(Equal([]*pluginapi.Device{
			{ID: "eni-slot-0", Health: pluginapi.Healthy},
			{ID: "eni-slot-1", Health: pluginapi.Healthy},
			{ID: "eni-slot-2", Health: pluginapi.Healthy},
		}))
	})

	It("returns no devices for zero slots", func() {
		Expect(healthyDevices(0)).To(BeEmpty())
	})
})
