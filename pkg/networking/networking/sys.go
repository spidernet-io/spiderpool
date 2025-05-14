package networking

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	SysClassNetDevicePath   = "/sys/class/net"
	SysVirtualNetDevicePath = "/sys/devices/virtual/net"
	SysBusPciDevicesPath    = "/sys/bus/pci/devices"
	SysDevicePciPath        = "/sys/devices/pci"
)

// PCI device class: https://admin.pci-ids.ucw.cz/read/PD/
var (
	ETH_DEVICE_CLASS        = "0x020000"
	INFINIBAND_DEVICE_CLASS = "0x020700"
	GPU_DEVICE_CLASS        = "0x030200"
	GPU1_DEVICE_CLASS       = "0x038000"
)

// IsVirtualInterface checks if the interface is virtual or not by
// checking if the /sys/devices/virtual/net/{ifName} exists
func IsVirtualNetDevice(ifName string) (bool, error) {
	devicePath := path.Join(SysVirtualNetDevicePath, ifName)
	_, err := os.Lstat(devicePath)
	if err == nil {
		return true, nil
	}

	// if !os.IsNotExist(err) {
	// 	return false, err
	// }
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
	datas, err := GetSysDeviceConfigForNetDev(ifName, "device")
	if err != nil {
		return "", err
	}

	return datas, nil
}

func GetPciVendorForNetDev(ifName string) (string, error) {
	datas, err := GetSysDeviceConfigForNetDev(ifName, "vendor")
	if err != nil {
		return "", err
	}

	return datas, nil
}

// GetSriovTotalVfsForNetDev get sriov vf count from sysfs
func GetSriovTotalVfsForNetDev(ifName string) (int, error) {
	totalvfsBytes, err := GetSysDeviceConfigForNetDev(ifName, "sriov_totalvfs")
	if err != nil {
		return 0, err
	}

	vfs, err := strconv.Atoi(totalvfsBytes)
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
	_, err := GetSysDeviceConfigForNetDev(iface, "sriov_totalvfs")
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

func GetSysDeviceConfigForNetDev(iface, attribute string) (string, error) {
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

func GetSysDeviceConfigForPciDev(dev, attribute string) (string, error) {
	path := fmt.Sprintf("%s/%s/%s", SysBusPciDevicesPath, dev, attribute)
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

// GetSriovAvailableVfPciAddressesForNetDev returns the list of available VF PCI addresses for
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
		availableVfPciAddresses = append(availableVfPciAddresses, getShortPciAddress(vfPciAddr))
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
		dirInfo, err := os.Lstat(dir)
		if err == nil && (dirInfo.Mode()&os.ModeSymlink != 0) {
			linkName, err := filepath.EvalSymlinks(dir)
			if err == nil {
				vfLink := filepath.Base(linkName)
				vfList = append(vfList, getShortPciAddress(vfLink))
			}
		}
	}
	return
}

// getShortPciAddress returns the short form of a PCI address
// [domain]:[bus]:[device].[function] -> [bus]:[device].[function]
// e.g. 0000:af:00.1 -> af:00.1
func getShortPciAddress(pciAddress string) string {
	parts := strings.Split(pciAddress, ":")
	if len(parts) == 3 {
		// parts[0] is domain
		// parts[1] is bus
		// parts[2] include device.function
		return parts[2]
	}
	return ""
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

// getPciPathFromReadLink returns the PCI path from the readlink of the PCI bus path
// e.g. 0000:81:00.0 -> ../../../devices/pci0000:80/0000:80:00.0/0000:81:00.0
// return pci0000:80/0000:80:00.0/0000:81:00.0
func getPciPathFromReadLink(pciBusPath string) (string, error) {
	// found the gpu, then we check the pci affinity with the net device
	pciDevicePath, err := filepath.EvalSymlinks(pciBusPath)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(pciDevicePath, "/sys/devices/"), nil
}

// GetGdrGpusForNetDevice returns the list of GPUs that are connected to the same host bridge as the network device
func GetGdrGpusForNetDevice(ifName string) (gdrGpus []string, err error) {
	// Get PCI address for the network device
	netDevicePciAddress := fmt.Sprintf("%s/%s/device", SysClassNetDevicePath, ifName)
	if _, err := os.Lstat(netDevicePciAddress); err != nil {
		return nil, err
	}

	// get full pci address, e.g. 0000:81:00.0 -> pci0000:00/0000:00:02.0/0000:02:00.0/0000:03:08.0/0000:05:00.0
	netDeviceFullPciAddress, err := getPciPathFromReadLink(netDevicePciAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get full PCI address for network device %s: %v", ifName, err)
	}

	// get numa node
	netDeviceNumaNode, err := GetSysDeviceConfigForNetDev(ifName, "numa_node")
	if err != nil {
		return nil, fmt.Errorf("failed to get NUMA node for network device %s: %v", ifName, err)
	}

	netDeviceFullPciAddressSlice := strings.Split(netDeviceFullPciAddress, "/")

	pciAddress, err := os.ReadDir(SysBusPciDevicesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", SysBusPciDevicesPath, err)
	}

	for _, dir := range pciAddress {
		gpuPciBusPath := filepath.Join(SysBusPciDevicesPath, dir.Name())
		classBytes, err := os.ReadFile(gpuPciBusPath + "/class")
		if err != nil {
			continue
		}

		classStr := string(bytes.TrimSpace(classBytes))
		if (classStr != GPU_DEVICE_CLASS) && (classStr != GPU1_DEVICE_CLASS) {
			continue
		}

		// get numa node
		gpuNumaNode, err := GetSysDeviceConfigForPciDev(dir.Name(), "numa_node")
		if err != nil {
			continue
		}

		if gpuNumaNode != netDeviceNumaNode {
			// GPU and network device are not in the same NUMA node, it's SYS topology
			continue
		}

		// found a gpu, then we check the pci affinity with the net device
		// like pci0000:ce/0000:ce:01.0/0000:cf:00.0/0000:d0:01.0/0000:d2:00.0
		gpuFullPciPath, err := getPciPathFromReadLink(gpuPciBusPath)
		if err != nil {
			continue
		}
		gpuFullPciPathSlice := strings.Split(gpuFullPciPath, "/")
		isGdrEnabled := comparePciAffinity(netDeviceFullPciAddressSlice, gpuFullPciPathSlice)
		if isGdrEnabled {
			gdrGpus = append(gdrGpus, dir.Name())
		}
	}

	return
}

// comparePciAffinity checks the PCI affinity between the network device and the GPU.
// isPIX: Connection traversing at most a single PCIe bridge
// isPXB: Connection traversing multiple PCIe bridges (without traversing the PCIe Host Bridge)
func comparePciAffinity(nicPciBusSlices, gpuPciBusSlices []string) (isGdr bool) {
	// corner case 1
	// if the two pci devices are directly connected to the same host bridge
	// or cross only one pcie bridge, which we consider to be a PIX topology.
	// pci1: 0000:80:00.0/81:00.0
	// pci2: 0000:80:00.0/81:00.1
	if len(nicPciBusSlices) < 2 || len(gpuPciBusSlices) < 2 {
		return false
	}

	// nic and gpu are not in the same host bridge
	if nicPciBusSlices[0] != gpuPciBusSlices[0] {
		return false
	}
	return true
}
