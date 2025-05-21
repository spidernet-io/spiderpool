package nri

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"go.uber.org/zap"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	NvidiaGPU = iota
	// More GPU vendor
)

var (
	NvidiaGPUResourceName = "nvidia.com"
	NvidiaDriverGPUPath   = "/proc/driver/nvidia/gpus"
)

type networkSupport struct {
	devName  string
	gpuCount int
	gpus     map[string]struct{}
}

func (n *nriPlugin) getAllocatedGpusForPodSandbox(ctx context.Context, pod *api.PodSandbox) (gpus []string, err error) {
	n.logger.Debug("Getting allocated GPUs for pod", zap.String("podID", pod.GetId()))

	// It shoule be better use Get function here, but we should enable the kubelet feature-gate
	// "KubeletPodResourcesGetAllocatable"(alpha in 1.27).
	// podResources, err := n.kubeletClient.Get(ctx, &podresourcesapi.GetPodResourcesRequest{
	// 	PodName:      pod.GetName(),
	// 	PodNamespace: pod.GetNamespace(),
	// })\
	resp, err := n.kubeletClient.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		n.logger.Error("Failed to get pod resource map", zap.Error(err))
		return
	}

	for _, r := range resp.PodResources {
		if r.Name == pod.Name && r.Namespace == pod.Namespace {
			return n.getPodAllocatedGpuResources(pod, r)
		}
	}

	// return if no any resources allocated
	return
}

func (n *nriPlugin) getPodAllocatedGpuResources(sandbox *api.PodSandbox, PodResources *podresourcesapi.PodResources) ([]string, error) {
	var gpuType int
	var deviceUUIDs []string

	for _, c := range PodResources.Containers {
		for _, dev := range c.Devices {
			// TODO(@cyclinder): more GPU vendor
			if strings.HasPrefix(dev.ResourceName, NvidiaGPUResourceName) {
				// Found Nvidia GPU Resources
				gpuType = NvidiaGPU
				deviceUUIDs = append(deviceUUIDs, dev.DeviceIds...)
			}
		}
	}

	if len(deviceUUIDs) == 0 {
		return []string{}, nil
	}

	var gpusDevicePciAddr []string
	switch gpuType {
	case NvidiaGPU:
		n.logger.Debug("NVIDIA GPU resources allocated to pod",
			zap.Strings("gpuUUIDs", deviceUUIDs),
			zap.String("podName", sandbox.GetName()),
			zap.String("namespace", sandbox.GetNamespace()))

		allNvidiaGpuMap, err := GetAllNvidiaGpusMap()
		if err != nil {
			n.logger.Warn("Failed to get GPU map", zap.Error(err))
		}

		for _, uuid := range deviceUUIDs {
			if allNvidiaGpuMap[uuid] != "" {
				gpusDevicePciAddr = append(gpusDevicePciAddr, allNvidiaGpuMap[uuid])
			}
		}
	}

	return gpusDevicePciAddr, nil
}

// filterPfToCniConfigsWithGpuRdmaAffinity filters the CNI configs for the given GPUs, return the pf name to cni config map
func filterPfToCniConfigsWithGpuRdmaAffinity(gpus []string, resourceSlice *resourcev1beta1.ResourceSlice) map[string]string {
	if len(gpus) == 0 {
		return nil
	}

	// Map to track network configurations found for each GPU
	gpuNetworkMap := make(map[string][]string)
	// Map to track which GPUs each network interface supports
	networkGpuMap := make(map[string]map[string]struct{})
	// Map to store device name to CNI config mapping
	deviceNameToCniConfig := make(map[string]string)

	// Step 1: Collect all available network interface CNI configurations for each GPU
	for _, dev := range resourceSlice.Spec.Devices {
		if dev.Basic == nil || dev.Basic.Attributes == nil {
			continue
		}

		if !IsReadyRdmaResourceDevice(dev.Basic) {
			continue
		}

		// Get CNI configuration for this network interface
		// cniConfigsStr maybe be more than one
		cniConfigsStr := GetStringValueForAttributes("cniConfigs", dev.Basic.Attributes)
		if cniConfigsStr == "" {
			continue
		}

		// Get GPU affinity for this network interface
		gpusInAttribute := GetStringValueForAttributes("gdrAffinityGpus", dev.Basic.Attributes)
		if gpusInAttribute == "" {
			continue
		}

		// Store device name to CNI config mapping
		deviceNameToCniConfig[dev.Name] = cniConfigsStr

		// Initialize the map for this network interface if not already done
		if _, exists := networkGpuMap[dev.Name]; !exists {
			networkGpuMap[dev.Name] = make(map[string]struct{})
		}

		// Check if each requested GPU has affinity with this network interface
		for _, gpu := range gpus {
			if strings.Contains(gpusInAttribute, gpu) {
				// Add this network interface's name to the corresponding GPU's config list
				gpuNetworkMap[gpu] = append(gpuNetworkMap[gpu], dev.Name)
				// Record that this network interface supports this GPU
				networkGpuMap[dev.Name][gpu] = struct{}{}
			}
		}
	}

	// Result map: network interface name -> CNI config
	result := make(map[string]string)

	// Step 2: Check if any network interface supports all GPUs
	for devName, supportedGpus := range networkGpuMap {
		if len(supportedGpus) == len(gpus) {
			// This network interface supports all GPUs
			allGpusSupported := true
			for _, gpu := range gpus {
				if _, exists := supportedGpus[gpu]; !exists {
					allGpusSupported = false
					break
				}
			}
			if allGpusSupported {
				// Return a map with just this network interface
				result[devName] = deviceNameToCniConfig[devName]
				return result
			}
		}
	}

	// Step 3: If no network interface supports all GPUs, we need to find a combination of networks
	// that can cover all GPUs with minimal number of networks

	// First, try to find networks that support multiple GPUs
	var coveredGpus = make(map[string]struct{})
	var selectedDevices = make(map[string]struct{})

	// Sort networks by the number of GPUs they support (descending)
	var networkSupports []networkSupport
	for devName, supportedGpus := range networkGpuMap {
		networkSupports = append(networkSupports, networkSupport{
			devName:  devName,
			gpuCount: len(supportedGpus),
			gpus:     supportedGpus,
		})
	}

	// Sort by GPU count descending
	sort.Slice(networkSupports, func(i, j int) bool {
		return networkSupports[i].gpuCount > networkSupports[j].gpuCount
	})

	// Greedily select networks that cover the most uncovered GPUs
	for len(coveredGpus) < len(gpus) && len(networkSupports) > 0 {
		// Find the network that covers the most uncovered GPUs
		bestIdx := -1
		bestNewCoverage := 0

		for i, ns := range networkSupports {
			// Count how many new GPUs this network would cover
			newCoverage := 0
			for gpu := range ns.gpus {
				if _, covered := coveredGpus[gpu]; !covered {
					newCoverage++
				}
			}

			if newCoverage > bestNewCoverage {
				bestNewCoverage = newCoverage
				bestIdx = i
			}
		}

		// If we couldn't find a network that covers new GPUs, break
		if bestIdx == -1 || bestNewCoverage == 0 {
			break
		}

		// Add the selected network
		selected := networkSupports[bestIdx]
		if _, exists := selectedDevices[selected.devName]; !exists {
			selectedDevices[selected.devName] = struct{}{}
			result[selected.devName] = deviceNameToCniConfig[selected.devName]
		}

		// Mark the GPUs as covered
		for gpu := range selected.gpus {
			coveredGpus[gpu] = struct{}{}
		}

		// Remove the selected network from consideration
		networkSupports = slices.Delete(networkSupports, bestIdx, bestIdx+1)
	}

	// If we've covered all GPUs, return the selected configs
	if len(coveredGpus) == len(gpus) {
		return result
	}

	// Step 4: If no single network interface can support all GPUs, select one for each GPU
	for _, gpu := range gpus {
		devNames, found := gpuNetworkMap[gpu]
		if !found || len(devNames) == 0 {
			continue
		}

		// Add the first configuration if not already added
		devName := devNames[0]
		if _, exists := selectedDevices[devName]; !exists {
			selectedDevices[devName] = struct{}{}
			result[devName] = deviceNameToCniConfig[devName]
		}
	}

	return result
}
