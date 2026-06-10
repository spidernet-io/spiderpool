// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"fmt"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func slotDeviceID(index int) string {
	return fmt.Sprintf("sub-eni-%d", index)
}

func healthyDevices(maxSlots int) []*pluginapi.Device {
	if maxSlots <= 0 {
		return nil
	}

	devices := make([]*pluginapi.Device, 0, maxSlots)
	for i := 0; i < maxSlots; i++ {
		devices = append(devices, &pluginapi.Device{
			ID:     slotDeviceID(i),
			Health: pluginapi.Healthy,
		})
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
