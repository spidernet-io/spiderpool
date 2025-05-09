package nri

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	defaultKubeletSocket       = "kubelet.sock" // which is defined in k8s.io/kubernetes/pkg/kubelet/apis/podresources
	kubeletConnectionTimeout   = 10 * time.Second
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	defaultPodResourcesPath    = "/var/lib/kubelet/pod-resources"
	unixProtocol               = "unix"
)

// GetResourceClient returns an instance of ResourceClient interface initialized with Pod resource information
func GetKubeletResourceClient() (podresourcesapi.PodResourcesListerClient, *grpc.ClientConn, error) {
	kubeletSocketPath := filepath.Join(defaultPodResourcesPath, defaultKubeletSocket)
	if !hasKubeletAPIEndpoint(kubeletSocketPath) {
		return nil, nil, fmt.Errorf("GetResourceClient: no Kubelet resource API endpoint found")
	}

	return getKubeletResourceClient(localEndpoint(kubeletSocketPath))
}

// LocalEndpoint returns the full path to a unix socket at the given endpoint
// which is in k8s.io/kubernetes/pkg/kubelet/util
func localEndpoint(path string) string {
	return unixProtocol + ":" + path
}

func getKubeletResourceClient(kubeletSocketURL string) (podresourcesapi.PodResourcesListerClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(kubeletSocketURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaultPodResourcesMaxSize)))
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing socket %s: %v", kubeletSocketURL, err)
	}
	return podresourcesapi.NewPodResourcesListerClient(conn), conn, nil
}

func hasKubeletAPIEndpoint(path string) bool {
	// Check for kubelet resource API socket file
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// GetAllNvidiaGpusMap returns a map of GPU index to UUID
func GetAllNvidiaGpusMap() (map[string]string, error) {
	nvidiaGpuDirs, err := os.ReadDir(NvidiaDriverGPUPath)
	if err != nil {
		return nil, err
	}

	// Map GPU index/PCI address to UUID
	gpuMap := make(map[string]string, len(nvidiaGpuDirs))
	for _, d := range nvidiaGpuDirs {
		if !d.IsDir() {
			continue
		}

		gpuInfoPath := filepath.Join(NvidiaDriverGPUPath, d.Name(), "information")

		// Read the information file
		content, err := os.ReadFile(gpuInfoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read GPU information file %s: %v", gpuInfoPath, err)
		}

		// Parse the content to extract UUID
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "GPU UUID") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					uuid := strings.TrimSpace(parts[1])
					// Store both index->UUID and PCI->UUID mappings
					gpuMap[uuid] = d.Name()
				}
				break
			}
		}
	}

	return gpuMap, nil
}

func GetAllocatedNvidiaGpusFromPodNamespace(containerPID int, filePath string) ([]string, error) {
	return []string{}, nil
}
