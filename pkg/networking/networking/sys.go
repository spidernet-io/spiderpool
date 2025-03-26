package networking

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spidernet-io/spiderpool/pkg/utils"
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

func GetGpuAffinityForNetDevice(ifName string) (pixGpus, pxbGpus []string, err error) {
	// Get PCI address for the network device
	netDevicePciAddress, err := GetPciAddessForNetDev(ifName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get PCI address for network device %s: %v", ifName, err)
	}

	netDeviceFullPciAddress, err := getPciPathFromReadLink(netDevicePciAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get full PCI address for network device %s: %v", ifName, err)
	}

	netDeviceFullPciAddressSlice := strings.Split(netDeviceFullPciAddress, "/")

	pciAddress, err := os.ReadDir(SysBusPciDevicesPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", SysBusPciDevicesPath, err)
	}

	for _, dir := range pciAddress {
		if !dir.IsDir() {
			continue
		}

		gpuPciBusPath := filepath.Join(SysBusPciDevicesPath, dir.Name())
		classBytes, err := os.ReadFile(gpuPciBusPath + "/class")
		if err != nil {
			continue
		}

		classStr := string(bytes.TrimSpace(classBytes))
		if classStr != GPU_DEVICE_CLASS && classStr != GPU1_DEVICE_CLASS {
			continue
		}

		// found a gpu, then we check the pci affinity with the net device
		gpuFullPciPath, err := getPciPathFromReadLink(gpuPciBusPath)
		if err != nil {
			continue
		}
		gpuFullPciPathSlice := strings.Split(gpuFullPciPath, "/")

		isPix, isPxb := comparePciAffinity(netDeviceFullPciAddressSlice, gpuFullPciPathSlice)
		if isPix {
			pixGpus = append(pixGpus, gpuFullPciPathSlice[len(gpuFullPciPathSlice)-1])
		} else if isPxb {
			pxbGpus = append(pxbGpus, gpuFullPciPathSlice[len(gpuFullPciPathSlice)-1])
		}
	}

	return
}

// comparePciAffinity checks the PCI affinity between the network device and the GPU.
// isPIX: Connection traversing at most a single PCIe bridge
// isPXB: Connection traversing multiple PCIe bridges (without traversing the PCIe Host Bridge)
func comparePciAffinity(nicPciBusSlices, gpuPciBusSlices []string) (isPIX, isPXB bool) {
	// corner case 1
	// if the two pci devices are directly connected to the same host bridge
	// or cross only one pcie bridge, which we consider to be a PIX topology.
	// pci1: 0000:80:00.0/81:00.0
	// pci2: 0000:80:00.0/81:00.1
	if len(nicPciBusSlices) < 2 || len(gpuPciBusSlices) < 2 {
		return false, false
	}

	// nic and gpu are not in the same host bridge
	if nicPciBusSlices[0] != gpuPciBusSlices[0] {
		return false, false
	}

	// PIX case 1: Both devices are directly connected to the same bridge with the same path length
	if len(nicPciBusSlices) == 2 && len(gpuPciBusSlices) == 2 {
		return true, false
	}

	// PIX case 2:
	// if one pci device is connected to the host bridge directly, and another
	// pci device is connected to host bridge across only one pcie bridge, which we consider
	// to be a PIX topology.
	// pci1: 0000:00:03.0/09:00.0/0d:01.0
	// pci2: 0000:00:03.0/0d:02.0
	// or
	// pci1: 0000:00:03.0/0d:02.0
	// pci2: 0000:00:03.0/09:00.0/0d:01.0
	if (len(nicPciBusSlices) == 2 && utils.AbsInt(len(nicPciBusSlices), len(gpuPciBusSlices)) == 1) ||
		(len(gpuPciBusSlices) == 2 && utils.AbsInt(len(nicPciBusSlices), len(gpuPciBusSlices)) == 1) {
		return true, false
	}

	// Different first level PCIe bridges under the same root bridge
	// Usually indicates PHB topology relationship
	if nicPciBusSlices[1] != gpuPciBusSlices[1] {
		return false, false
	}

	// PIX case 3:
	// Connection traversing at most a single PCIe bridge
	// pci1: pci0000:00/0000:00:02.0/0000:02:00.0/0000:03:04.0/0000:04:00.0
	// pci2: pci0000:00/0000:00:02.0/0000:02:00.0/0000:03:04.0/0000:05:00.0
	// Check if the second last components in both paths are the same
	if len(nicPciBusSlices) >= 3 && len(gpuPciBusSlices) >= 3 &&
		nicPciBusSlices[len(nicPciBusSlices)-2] == gpuPciBusSlices[len(gpuPciBusSlices)-2] {
		return true, false
	}

	// PIX case 4:
	// Devices are connected through a common bridge three levels up
	if len(nicPciBusSlices) >= 4 && len(gpuPciBusSlices) >= 4 &&
		nicPciBusSlices[len(nicPciBusSlices)-3] == gpuPciBusSlices[len(gpuPciBusSlices)-3] {
		return true, false
	}

	// PXB case:
	// Connection traversing multiple PCIe bridges (without traversing the PCIe Host Bridge)
	// pci1: pci0000:00/0000:00:02.0/0000:02:00.0/0000:03:04.0/0000:04:00.0
	// pci2: pci0000:00/0000:00:02.0/0000:01:00.0/0000:03:08.0/0000:05:00.0
	// Devices share the same root but traverse different paths
	return false, true
}
