package rdmamap

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/vishvananda/netns"
)

type RdmaStatEntry struct {
	Name  string
	Value uint64
}

type RdmaPortStats struct {
	HwStats []RdmaStatEntry /* /sys/class/infiniband/<dev>/<port>/hw_counters */
	Stats   []RdmaStatEntry /* /sys/class/infiniband/<dev>/<port>/counters */
	Port    int
}

type RdmaStats struct {
	PortStats []RdmaPortStats
}

func readCounter(name string) uint64 {
	fd, err := os.OpenFile(name, os.O_RDONLY, 0444)
	if err != nil {
		return 0
	}
	defer fd.Close()

	if _, err = fd.Seek(0, io.SeekStart); err != nil {
		return 0
	}

	data, err := ioutil.ReadAll(fd)
	if err != nil {
		return 0
	}
	dataStr := string(data)
	dataStr = strings.Trim(dataStr, "\n")
	value, _ := strconv.ParseUint(dataStr, 10, 64)
	return value
}

//nolint:prealloc
func getCountersFromDir(path string) ([]RdmaStatEntry, error) {
	var stats []RdmaStatEntry

	fd, err := os.Open(path)
	if err != nil {
		return stats, err
	}
	defer fd.Close()

	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return stats, err
	}

	for _, file := range fileInfos {
		if file.IsDir() {
			continue
		}
		value := readCounter(filepath.Join(path, file.Name()))
		entry := RdmaStatEntry{file.Name(), value}
		stats = append(stats, entry)
	}
	return stats, nil
}

// Get RDMA Sysfs stats from counters directory of a port of a rdma device
// Port number starts from 1.
func GetRdmaSysfsStats(rdmaDevice string, port int) ([]RdmaStatEntry, error) {
	path := filepath.Join(RdmaClassDir, rdmaDevice,
		RdmaPortsdir, strconv.Itoa(port), RdmaCountersDir)

	rdmastats, err := getCountersFromDir(path)
	return rdmastats, err
}

// Get RDMA Sysfs stats from hw_counters directory of a port of a rdma device
// Port number starts from 1.
func GetRdmaSysfsHwStats(rdmaDevice string, port int) ([]RdmaStatEntry, error) {
	path := filepath.Join(RdmaClassDir, rdmaDevice,
		RdmaPortsdir, strconv.Itoa(port), RdmaHwCountersDir)

	rdmastats, err := getCountersFromDir(path)
	return rdmastats, err
}

// Get RDMA sysfs starts from counter and hw_counters directory for a requested
// port of a device.
func GetRdmaSysfsAllStats(rdmaDevice string, port int) (RdmaPortStats, error) {
	var portstats RdmaPortStats

	hwstats, err := GetRdmaSysfsHwStats(rdmaDevice, port)
	if err != nil {
		return portstats, nil
	}
	portstats.HwStats = hwstats

	stats, err := GetRdmaSysfsStats(rdmaDevice, port)
	if err != nil {
		return portstats, nil
	}
	portstats.Stats = stats
	portstats.Port = port
	return portstats, nil
}

// Get RDMA sysfs starts from counter and hw_counters directory for a
// rdma device.
func GetRdmaSysfsAllPortsStats(rdmaDevice string) (RdmaStats, error) {
	var allstats RdmaStats

	path := filepath.Join(RdmaClassDir, rdmaDevice, RdmaPortsdir)
	fd, err := os.Open(path)
	if err != nil {
		return allstats, err
	}
	defer fd.Close()

	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return allstats, err
	}

	for i, file := range fileInfos {
		if fileInfos[i].Name() == "." || fileInfos[i].Name() == ".." {
			continue
		}
		if !file.IsDir() {
			continue
		}
		port, _ := strconv.Atoi(file.Name())
		portstats, err := GetRdmaSysfsAllStats(rdmaDevice, port)
		if err != nil {
			return allstats, err
		}
		allstats.PortStats = append(allstats.PortStats, portstats)
	}
	return allstats, nil
}

func printRdmaStats(device string, stats *RdmaStats) {
	for _, portstats := range stats.PortStats {
		fmt.Printf("device: %s, port: %d\n", device, portstats.Port)
		fmt.Println("Hw stats:")
		for _, entry := range portstats.HwStats {
			fmt.Printf("%s: %d\n", entry.Name, entry.Value)
		}
		fmt.Println("Stats:")
		for _, entry := range portstats.Stats {
			fmt.Printf("%s: %d\n", entry.Name, entry.Value)
		}
	}
}

// Get RDMA statistics of a docker container.
// containerID is prefixed matched against the running docker containers,
// so a non ambiguous short identifier can be supplied as well.
func GetDockerContainerRdmaStats(containerID string) {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	originalHandle, err := netns.Get()
	if err != nil {
		log.Println("Fail to get handle of current net ns", err)
		return
	}

	nsHandle, err := netns.GetFromDocker(containerID)
	if err != nil {
		log.Println("Invalid docker id: ", containerID)
		return
	}
	if netns.Set(nsHandle) != nil {
		return
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		_ = netns.Set(originalHandle)
		return
	}
	log.Printf("Net Interfaces: %v\n", ifaces)
	for _, iface := range ifaces {
		if iface.Name == loopBackIfName {
			continue
		}
		rdmadev, err := GetRdmaDeviceForNetdevice(iface.Name)
		if err != nil {
			continue
		}
		rdmastats, err := GetRdmaSysfsAllPortsStats(rdmadev)
		if err != nil {
			log.Println("Fail to query device stats: ", err)
			continue
		}
		printRdmaStats(rdmadev, &rdmastats)
	}
	_ = netns.Set(originalHandle)
}
