// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var _ = Describe("network resource plugin devices", Label("networkresourceplugin_devices_test"), func() {
	It("builds stable sub-ENI and master NIC device IDs", func() {
		Expect(subENISlotDeviceID(3)).To(Equal("sub-eni-3"))
		Expect(masterNICDeviceID("eth0", 4)).To(Equal("eth0-nic-4"))
	})

	It("returns no devices for non-positive counts", func() {
		Expect(healthyDevices(0, subENISlotDeviceID)).To(BeNil())
		Expect(healthyDevices(-1, subENISlotDeviceID)).To(BeNil())
	})

	It("builds healthy sub-ENI devices", func() {
		devices := healthySubENIDevices(2)

		Expect(devices).To(Equal([]*pluginapi.Device{
			{ID: "sub-eni-0", Health: pluginapi.Healthy},
			{ID: "sub-eni-1", Health: pluginapi.Healthy},
		}))
	})

	It("builds virtual master NIC devices", func() {
		devices := healthyMasterNICDevices("eth0", 3)

		Expect(devices).To(HaveLen(3))
		Expect(devices[0]).To(Equal(&pluginapi.Device{ID: "eth0-nic-0", Health: pluginapi.Healthy}))
		Expect(devices[2]).To(Equal(&pluginapi.Device{ID: "eth0-nic-2", Health: pluginapi.Healthy}))
	})

	It("builds device ID sets and ignores nil devices", func() {
		set := deviceIDSet([]*pluginapi.Device{
			{ID: "device-a", Health: pluginapi.Healthy},
			nil,
			{ID: "device-b", Health: pluginapi.Healthy},
		})

		Expect(set).To(Equal(map[string]struct{}{
			"device-a": {},
			"device-b": {},
		}))
	})
})
