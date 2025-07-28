// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

const (
	SysClassNetDevicePath = "/sys/class/net"
	SysBusPciDevicesPath  = "/sys/bus/pci/devices"
)

func GetPfNameFromVfDeviceId(vfDeviceID string) (string, error) {
	pfDeviceID, err := GetPfDeviceIDFromVF(vfDeviceID)
	if err != nil {
		return "", err
	}

	return GetPfNameFromPfDeviceID(pfDeviceID)
}

func GetPfDeviceIDFromVF(vfDeviceID string) (string, error) {
	// First try the traditional approach via sysfs (works in host namespace)
	vf_physfn := path.Join(SysBusPciDevicesPath, vfDeviceID, "physfn")
	physfnInfo, err := os.Lstat(vf_physfn)
	if err != nil {
		return "", fmt.Errorf("failed to get physfn info for VF %s: %v", vfDeviceID, err)
	}

	if physfnInfo.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("physfn %s is not a symlink", vf_physfn)
	}

	// Read the path that the symlink points to
	physfnPath, err := os.Readlink(vf_physfn)
	if err != nil {
		return "", fmt.Errorf("failed to read physfn symlink for vf %s: %v", vfDeviceID, err)
	}

	return filepath.Base(physfnPath), nil
}

func GetPfNameFromPfDeviceID(pfDeviceID string) (string, error) {
	// Get the network interface name from PCI address
	pfNetDir := path.Join(SysBusPciDevicesPath, pfDeviceID, "net")
	dirs, err := os.ReadDir(pfNetDir)
	if err != nil {
		return "", fmt.Errorf("failed to read net directory for pf %s: %v", pfDeviceID, err)
	}

	if len(dirs) == 0 {
		return "", fmt.Errorf("no network interface found for pf %s", pfDeviceID)
	}

	return dirs[0].Name(), nil
}
