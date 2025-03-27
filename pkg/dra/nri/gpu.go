package nri

import (
	"context"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"go.uber.org/zap"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	NvidiaGPUResourceName = "nvidia.com/gpu"
)

func (n *nriPlugin) getAllocatedGpusForPodSandbox(ctx context.Context, pod *api.PodSandbox) (gpus []string, err error) {
	n.logger.Info("Getting allocated GPUs for pod", zap.String("podID", pod.GetId()))

	podResources, err := n.kubeletClient.Get(ctx, &podresourcesapi.GetPodResourcesRequest{
		PodName:      pod.GetName(),
		PodNamespace: pod.GetNamespace(),
	})
	if err != nil {
		n.logger.Error("Failed to get pod resource map", zap.Error(err))
		return
	}

	return GetPodAllocatedGpuResources(NvidiaGPUResourceName, podResources), nil
}

func (n *nriPlugin) getAllocatedGpusForContainer(contaienr *api.Container) (gpus []string, err error) {
	n.logger.Info("Debug GPU in CreateContainer", zap.Any("container", contaienr))
	return nil, nil
}

// 从设备路径中提取设备ID
func extractDeviceIDFromPath(path string) string {
	// 假设路径格式为 /dev/nvidia0
	parts := strings.Split(path, "nvidia")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
