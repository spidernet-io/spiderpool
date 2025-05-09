package nri

import (
	"context"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"go.uber.org/zap"
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
		n.logger.Debug("No GPU resources allocated to this pod",
			zap.String("podName", sandbox.GetName()),
			zap.String("namespace", sandbox.GetNamespace()))
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
