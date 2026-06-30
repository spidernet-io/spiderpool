// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"fmt"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const DefaultMasterNICMaxCount = 10000

func subENISlotDeviceID(index int) string {
	return fmt.Sprintf("sub-eni-%d", index)
}

func masterNICDeviceID(iface string, index int) string {
	return fmt.Sprintf("%s-nic-%d", iface, index)
}

func healthySubENIDevices(maxCount int) []*pluginapi.Device {
	return healthyDevices(maxCount, subENISlotDeviceID)
}

func healthyMasterNICDevices(iface string, maxCount int) []*pluginapi.Device {
	return healthyDevices(maxCount, func(index int) string {
		return masterNICDeviceID(iface, index)
	})
}

func healthyDevices(count int, id func(int) string) []*pluginapi.Device {
	if count <= 0 {
		return nil
	}
	devices := make([]*pluginapi.Device, 0, count)
	for i := 0; i < count; i++ {
		devices = append(devices, &pluginapi.Device{ID: id(i), Health: pluginapi.Healthy})
	}
	return devices
}

func deviceIDSet(devices []*pluginapi.Device) map[string]struct{} {
	result := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		if device == nil {
			continue
		}
		result[device.ID] = struct{}{}
	}
	return result
}
