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
			{ID: "sub-eni-0", Health: pluginapi.Healthy},
			{ID: "sub-eni-1", Health: pluginapi.Healthy},
			{ID: "sub-eni-2", Health: pluginapi.Healthy},
		}))
	})

	It("returns no devices for zero slots", func() {
		Expect(healthyDevices(0)).To(BeEmpty())
	})

	It("returns no devices for negative slots", func() {
		Expect(healthyDevices(-1)).To(BeNil())
	})

	It("generates stable slot ID strings by index", func() {
		Expect(slotDeviceID(0)).To(Equal("sub-eni-0"))
		Expect(slotDeviceID(5)).To(Equal("sub-eni-5"))
	})

	It("builds a device ID set from a device list", func() {
		devices := healthyDevices(3)
		set := deviceIDSet(devices)

		Expect(set).To(HaveLen(3))
		Expect(set).To(HaveKey("sub-eni-0"))
		Expect(set).To(HaveKey("sub-eni-1"))
		Expect(set).To(HaveKey("sub-eni-2"))
	})

	It("skips nil entries when building device ID set", func() {
		devices := []*pluginapi.Device{nil, {ID: "sub-eni-0"}, nil}
		set := deviceIDSet(devices)

		Expect(set).To(HaveLen(1))
		Expect(set).To(HaveKey("sub-eni-0"))
	})

	It("returns an empty set for a nil device list", func() {
		Expect(deviceIDSet(nil)).To(BeEmpty())
	})
})
