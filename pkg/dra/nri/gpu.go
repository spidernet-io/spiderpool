package nri

import (
	"strings"

	"github.com/containerd/nri/pkg/api"
	"go.uber.org/zap"
)

func (n *nriPlugin) getAllocatedGpusForPodSandbox(pod *api.PodSandbox) (gpus []string, err error) {
	n.logger.Info("Getting allocated GPUs for pod", zap.String("podID", pod.GetId()))

	n.logger.Info("Debug GPU in RunPodSandBox", zap.Any("podSandBox", pod))
	resourceMap, err := n.ck.GetPodResourceMap(pod.Uid, pod.Name, pod.Namespace)
	if err != nil {
		n.logger.Error("Failed to get pod resource map", zap.Error(err))
		return
	}
	n.logger.Info("Debug resourceMap in RunPodSandBox", zap.Any("resourceMap", resourceMap))
	// annotations := pod.GetAnnotations()
	// if gpuAnnotation, ok := annotations["nvidia.com/gpu"]; ok {
	// 	n.logger.Info("Found GPU annotation", zap.String("gpuAnnotation", gpuAnnotation))
	// 	// 解析注解中的GPU信息
	// 	return strings.Split(gpuAnnotation, ","), nil
	// }

	// // 方法2：从Linux命名空间中查找GPU设备
	// // 如果Pod有Linux命名空间信息
	// if pod.GetLinux() != nil {
	// 	// 这里可以实现通过命名空间查找GPU设备的逻辑
	// 	n.logger.Info("Checking Linux namespace for GPU devices",
	// 		zap.Uint32("podPID", pod.GetPid()))

	// 	// 实现从命名空间查找GPU设备的逻辑
	// 	// ...
	// }

	// // 方法3：从环境变量中获取
	// // 这需要在CreateContainer回调中实现，因为容器环境变量在PodSandbox中不可用

	// n.logger.Info("No GPU devices found for pod", zap.String("podID", pod.GetId()))
	return nil, nil
}

func (n *nriPlugin) getAllocatedGpusForContainer(contaienr *api.Container) (gpus []string, err error) {
	n.logger.Info("Debug GPU in CreateContainer", zap.Any("container", contaienr))
	// annotations := pod.GetAnnotations()
	// if gpuAnnotation, ok := annotations["nvidia.com/gpu"]; ok {
	// 	n.logger.Info("Found GPU annotation", zap.String("gpuAnnotation", gpuAnnotation))
	// 	// 解析注解中的GPU信息
	// 	return strings.Split(gpuAnnotation, ","), nil
	// }

	// // 方法2：从Linux命名空间中查找GPU设备
	// // 如果Pod有Linux命名空间信息
	// if pod.GetLinux() != nil {
	// 	// 这里可以实现通过命名空间查找GPU设备的逻辑
	// 	n.logger.Info("Checking Linux namespace for GPU devices",
	// 		zap.Uint32("podPID", pod.GetPid()))

	// 	// 实现从命名空间查找GPU设备的逻辑
	// 	// ...
	// }

	// // 方法3：从环境变量中获取
	// // 这需要在CreateContainer回调中实现，因为容器环境变量在PodSandbox中不可用

	// n.logger.Info("No GPU devices found for pod", zap.String("podID", pod.GetId()))
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
