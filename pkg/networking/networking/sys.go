package networking

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

var (
	SysClassNetDevicePath   = "/sys/class/net"
	SysVirtualNetDevicePath = "/sys/devices/virtual/net"
	SysBusPciDevicesPath    = "/sys/bus/pci/devices"
	SysDevicePciPath        = "/sys/devices/pci"
)

// IsVirtualInterface checks if the interface is virtual or not by
// checking if the /sys/devices/virtual/net/{ifName} exists
func IsVirtualNetDevice(ifName string) (bool, error) {
	devicePath := path.Join(SysVirtualNetDevicePath, ifName)
	_, err := os.Lstat(devicePath)
	if err == nil {
		return true, nil
	}

	if !os.IsNotExist(err) {
		return false, err
	}
	return false, nil
}

func GetPciAddessForNetDev(ifName string) (string, error) {
	// get pci info from sysfs
	pciPath := fmt.Sprintf("%s/%s/device", SysClassNetDevicePath, ifName)
	if _, err := os.Lstat(pciPath); err != nil {
		return "", err
	}

	// get pci address
	pciAddr, err := os.Readlink(pciPath)
	if err != nil {
		return "", err
	}

	return filepath.Base(pciAddr), nil
}

func GetPciDeviceIdForNetDev(ifName string) (string, error) {
	datas, err := getSysDeviceConfigForNetDev(ifName, "device")
	if err != nil {
		return "", err
	}

	return datas, nil
}

func GetPciVendorForNetDev(ifName string) (string, error) {
	datas, err := getSysDeviceConfigForNetDev(ifName, "vendor")
	if err != nil {
		return "", err
	}

	return datas, nil
}

// GetSriovTotalVfsForNetDev get sriov vf count from sysfs
func GetSriovTotalVfsForNetDev(ifName string) (int, error) {
	totalvfsBytes, err := getSysDeviceConfigForNetDev(ifName, "sriov_totalvfs")
	if err != nil {
		return 0, err
	}

	vfs, err := strconv.Atoi(string(totalvfsBytes))
	if err != nil {
		return 0, err
	}
	return vfs, nil
}

func SriovTotalVfsFromPciBus(pciAddress string) int {
	total, err := os.ReadFile(SysBusPciDevicesPath + "/" + pciAddress + "/" + "sriov_totalvfs")
	if err != nil {
		return 0
	}

	total = bytes.TrimSpace(total)
	t, err := strconv.Atoi(string(total))
	if err != nil {
		return 0
	}
	return t
}

func IsSriovPfForNetDev(iface string) (bool, error) {
	_, err := getSysDeviceConfigForNetDev(iface, "sriov_totalvfs")
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// IsSriovVfForNetDev checks if the netdev is sriov vf or not by checking if
// the /sys/class/net/{ifName}/device/physfn exists
func IsSriovVfForNetDev(iface string) bool {
	vf_physfn := path.Join(SysClassNetDevicePath, iface, "device", "physfn")
	_, err := os.Lstat(vf_physfn)
	if err != nil {
		return false
	}

	return true
}

func GetPfFromVF(vfName string) (string, error) {
	vf_physfn := path.Join(SysClassNetDevicePath, vfName, "device", "physfn")
	// Check if the physfn symlink exists
	physfnInfo, err := os.Lstat(vf_physfn)
	if err != nil {
		return "", fmt.Errorf("failed to get physfn info for vf %s: %v", vfName, err)
	}

	if physfnInfo.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("physfn %s is not a symlink", vf_physfn)
	}

	// Read the path that the symlink points to
	physfnPath, err := os.Readlink(vf_physfn)
	if err != nil {
		return "", fmt.Errorf("failed to read physfn symlink for vf %s: %v", vfName, err)
	}

	// Get the PF's PCI address (last path component)
	pfPciAddr := filepath.Base(physfnPath)

	// Get the network interface name from PCI address
	pfNetDir := path.Join(SysBusPciDevicesPath, pfPciAddr, "net")
	dirs, err := os.ReadDir(pfNetDir)
	if err != nil {
		return "", fmt.Errorf("failed to read net directory for pf %s: %v", pfPciAddr, err)
	}

	if len(dirs) == 0 {
		return "", fmt.Errorf("no network interface found for pf %s", pfPciAddr)
	}

	return dirs[0].Name(), nil
}

func IsSriovVfFromPciAddress(pciAddress string) (bool, error) {
	_, err := os.Stat(SysBusPciDevicesPath + "/" + pciAddress + "/" + "physfn")
	if err == nil {
		return true, nil
	}

	return false, err
}

func getSysDeviceConfigForNetDev(iface, attribute string) (string, error) {
	path := fmt.Sprintf("%s/%s/device/%s", SysClassNetDevicePath, iface, attribute)
	if _, err := os.Lstat(path); err != nil {
		return "", err
	}

	// read attribute
	attributeBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(attributeBytes)), nil
}

// GetSriovAvailableVfPciAddressesForNetDev turns the list of available VF PCI addresses for
// the given network device.
func GetSriovAvailableVfPciAddressesForNetDev(ifName string) ([]string, error) {
	// get total VFs
	totalVfs, err := GetSriovTotalVfsForNetDev(ifName)
	if err != nil {
		return nil, fmt.Errorf("failed to get total VFs for interface %s: %v", ifName, err)
	}

	pciAddress, err := GetPciAddessForNetDev(ifName)
	if err != nil {
		return nil, fmt.Errorf("failed to get PCI address for interface %s: %v", ifName, err)
	}

	availableVfPciAddresses := []string{}
	for i := 0; i < totalVfs; i++ {
		vfDir := fmt.Sprintf("virtfn%d", i)
		vfPath := path.Join(SysBusPciDevicesPath, pciAddress, vfDir)

		// check if VF directory exists
		if _, err := os.Stat(vfPath); os.IsNotExist(err) {
			continue
		}

		// get VF PCI address
		vfPciAddrPath, err := os.Readlink(vfPath)
		if err != nil {
			continue
		}
		vfPciAddr := filepath.Base(vfPciAddrPath)

		// check if net directory exists
		vfNetDir := path.Join(vfPath, "net")

		// if net directory does not exist, VF may be unavailable
		if _, err := os.Stat(vfNetDir); os.IsNotExist(err) {
			continue
		}

		files, err := os.ReadDir(vfNetDir)
		if err != nil {
			continue
		}

		// if the net directory is empty, VF is assigned to a net namespace
		if len(files) == 0 {
			continue
		}
		availableVfPciAddresses = append(availableVfPciAddresses, vfPciAddr)
	}

	return availableVfPciAddresses, nil
}

// GetVFList returns a List containing PCI addr for all VF discovered in a given PF
func GetVFList(pfPciAddr string) (vfList []string, err error) {
	vfList = make([]string, 0)
	pfDir := path.Join(SysBusPciDevicesPath, pfPciAddr)
	_, err = os.Stat(pfDir)
	if err != nil {
		err = fmt.Errorf("could not get PF directory information for device: %s, Err: %v", pfDir, err)
		return
	}

	vfDirs, err := filepath.Glob(filepath.Join(pfDir, "virtfn*"))
	if err != nil {
		err = fmt.Errorf("error reading VF directories %v", err)
		return
	}

	// Read all VF directory and get add VF PCI addr to the vfList
	for _, dir := range vfDirs {
		fmt.Printf("VF directory: %s\n", dir)
		dirInfo, err := os.Lstat(dir)
		if err == nil && (dirInfo.Mode()&os.ModeSymlink != 0) {
			linkName, err := filepath.EvalSymlinks(dir)
			if err == nil {
				vfLink := filepath.Base(linkName)
				vfList = append(vfList, vfLink)
			}
		}
	}
	return
}

// GetNetdevBandwidth retrieves the bandwidth of a network device in Mbps.
// Returns speed in Mbps and a bool indicating if the device supports duplex mode.
func GetNetdevBandwidth(ifName string) (int, error) {
	// Read speed from sysfs
	speedPath := fmt.Sprintf("%s/%s/speed", SysClassNetDevicePath, ifName)
	if _, err := os.Stat(speedPath); err != nil {
		return 0, fmt.Errorf("failed to stat speed path for %s: %v", ifName, err)
	}

	speedBytes, err := os.ReadFile(speedPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read speed for network device %s: %v", ifName, err)
	}

	// Convert speed from string to int
	speedStr := string(bytes.TrimSpace(speedBytes))
	speed, err := strconv.Atoi(speedStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse speed value '%s' for network device %s: %v", speedStr, ifName, err)
	}

	return speed, nil
}
