package nri

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Mellanox/rdmamap"
	"github.com/containerd/nri/pkg/api"

	"go.uber.org/zap"
)

const (
	// DeviceTrackerDir is the directory where device allocation records are stored
	DeviceTrackerDir = "/var/lib/spiderpool/device-tracker"
)

// DeviceAllocation represents an allocated device for a pod
type DeviceAllocation struct {
	PodUID       string       `json:"podUID"`
	PodNamespace string       `json:"podNamespace"`
	PodName      string       `json:"podName"`
	DeviceInfo   []DeviceInfo `json:"deviceInfo"`
}

var (
	trackerLock sync.Mutex
)

// InitDeviceTracker initializes the device tracker
func InitDeviceTracker() error {
	// Create the device tracker directory if it doesn't exist
	if err := os.MkdirAll(DeviceTrackerDir, 0755); err != nil {
		return fmt.Errorf("failed to create device tracker directory: %v", err)
	}
	return nil
}

// GetDeviceAllocationFilePath returns the path to the device allocation file for a pod
func GetDeviceAllocationFilePath(podUID string) string {
	return filepath.Join(DeviceTrackerDir, fmt.Sprintf("%s.json", podUID))
}

// SaveDeviceAllocation saves the device allocation for a pod
func SaveDeviceAllocation(logger *zap.Logger, allocation *DeviceAllocation) error {
	trackerLock.Lock()
	defer trackerLock.Unlock()

	filePath := GetDeviceAllocationFilePath(allocation.PodUID)

	// Marshal the allocation to JSON
	data, err := json.Marshal(allocation)
	if err != nil {
		logger.Error("Failed to marshal device allocation", zap.Error(err))
		return fmt.Errorf("failed to marshal device allocation: %v", err)
	}

	// Write the allocation to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logger.Error("Failed to write device allocation file", zap.String("path", filePath), zap.Error(err))
		return fmt.Errorf("failed to write device allocation file: %v", err)
	}

	logger.Debug("Saved device allocation",
		zap.String("podUID", allocation.PodUID),
		zap.String("podNamespace", allocation.PodNamespace),
		zap.String("podName", allocation.PodName))
	return nil
}

// GetDeviceAllocation gets the device allocation for a pod
func GetDeviceAllocation(logger *zap.Logger, podUID string) (*DeviceAllocation, error) {
	trackerLock.Lock()
	defer trackerLock.Unlock()

	filePath := GetDeviceAllocationFilePath(podUID)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Debug("Device allocation file does not exist", zap.String("path", filePath))
		return nil, nil
	}

	// Read the allocation from file
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("Failed to read device allocation file", zap.String("path", filePath), zap.Error(err))
		return nil, fmt.Errorf("failed to read device allocation file: %v", err)
	}

	// Unmarshal the allocation from JSON
	var allocation DeviceAllocation
	if err := json.Unmarshal(data, &allocation); err != nil {
		logger.Error("Failed to unmarshal device allocation", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal device allocation: %v", err)
	}

	logger.Debug("Retrieved device allocation",
		zap.String("podUID", allocation.PodUID),
		zap.String("podNamespace", allocation.PodNamespace),
		zap.String("podName", allocation.PodName))
	return &allocation, nil
}

// DeleteDeviceAllocation deletes the device allocation for a pod
func DeleteDeviceAllocation(logger *zap.Logger, podUID string) error {
	trackerLock.Lock()
	defer trackerLock.Unlock()

	filePath := GetDeviceAllocationFilePath(podUID)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Debug("Device allocation file does not exist", zap.String("path", filePath))
		return nil
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		logger.Error("Failed to delete device allocation file", zap.String("path", filePath), zap.Error(err))
		return fmt.Errorf("failed to delete device allocation file: %v", err)
	}

	logger.Debug("Deleted device allocation", zap.String("podUID", podUID))
	return nil
}

// ParseRDMACharDevicesToMounts converts RDMA character devices to NRI Mount resources
func ParseRDMACharDevicesToMounts(deviceInfo []DeviceInfo) []*api.Mount {
	if len(deviceInfo) == 0 {
		return []*api.Mount{}
	}

	mounts := make([]*api.Mount, 0, len(deviceInfo)*4)

	// Add each RDMA character device as a mount
	for _, d := range deviceInfo {
		for _, charDevice := range d.RdmaCharDevices {
			if charDevice == rdmamap.RdmaUcmDevice {
				continue
			}

			// Create a mount for the device
			mount := &api.Mount{
				Source:      charDevice,
				Destination: charDevice,
				Type:        "bind",
				Options:     []string{"rbind", "rw"},
			}
			mounts = append(mounts, mount)
		}
	}

	// Add the RDMA CM device if it's not already included
	mounts = append(mounts, &api.Mount{
		Source:      rdmamap.RdmaUcmDevice,
		Destination: rdmamap.RdmaUcmDevice,
		Type:        "bind",
		Options:     []string{"rbind", "rw"},
	})

	return mounts
}
