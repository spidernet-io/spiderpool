package nri

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	checkPointfile = "/var/lib/kubelet/device-plugins/kubelet_internal_checkpoint"
)

// ResourceInfo is struct to hold Pod device allocation information
type ResourceInfo struct {
	Index     int
	DeviceIDs []string
}

// ResourceClient provides a kubelet Pod resource handle
type ResourceClient interface {
	// GetPodResourceMap returns an instance of a map of Pod ResourceInfo given a (Pod name, namespace) tuple
	GetPodResourceMap(string, string, string) (map[string]*ResourceInfo, error)
}

// PodDevicesEntry maps PodUID, resource name and allocated device id
type PodDevicesEntry struct {
	PodUID        string
	ContainerName string
	ResourceName  string
	DeviceIDs     map[int64][]string
	AllocResp     []byte
}

type checkpointData struct {
	PodDeviceEntries  []PodDevicesEntry
	RegisteredDevices map[string][]string
}

type checkpointFileData struct {
	Data     checkpointData
	Checksum uint64
}

type checkpoint struct {
	fileName   string
	podEntires []PodDevicesEntry
}

// GetCheckpoint returns an instance of Checkpoint
func GetCheckpoint() (ResourceClient, error) {
	return getCheckpoint(checkPointfile)
}

func getCheckpoint(filePath string) (ResourceClient, error) {
	cp := &checkpoint{fileName: filePath}
	err := cp.getPodEntries()
	if err != nil {
		return nil, err
	}
	return cp, nil
}

// getPodEntries gets all Pod device allocation entries from checkpoint file
func (cp *checkpoint) getPodEntries() error {

	cpd := &checkpointFileData{}
	rawBytes, err := os.ReadFile(cp.fileName)
	if err != nil {
		return fmt.Errorf("getPodEntries: error reading file %s\n%v\n", checkPointfile, err)
	}

	if err = json.Unmarshal(rawBytes, cpd); err != nil {
		return fmt.Errorf("getPodEntries: error unmarshalling raw bytes %v", err)
	}

	cp.podEntires = cpd.Data.PodDeviceEntries
	return nil
}

// GetPodResourceMap returns an instance of a map of ResourceInfo
func (cp *checkpoint) GetPodResourceMap(podUid, podName, podNamaspace string) (map[string]*ResourceInfo, error) {
	resourceMap := make(map[string]*ResourceInfo)

	if podUid == "" {
		return nil, fmt.Errorf("GetPodResourceMap: invalid Pod cannot be empty")
	}
	for _, pod := range cp.podEntires {
		if pod.PodUID == podUid {
			entry, ok := resourceMap[pod.ResourceName]
			if !ok {
				// new entry
				entry = &ResourceInfo{}
				resourceMap[pod.ResourceName] = entry
			}
			for _, v := range pod.DeviceIDs {
				// already exists; append to it
				entry.DeviceIDs = append(entry.DeviceIDs, v...)
			}
		}
	}
	return resourceMap, nil
}
