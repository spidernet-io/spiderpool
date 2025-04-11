package nri

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	defaultKubeletSocket       = "kubelet" // which is defined in k8s.io/kubernetes/pkg/kubelet/apis/podresources
	kubeletConnectionTimeout   = 10 * time.Second
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	defaultPodResourcesPath    = "/var/lib/kubelet/pod-resources"
	unixProtocol               = "unix"
)

func GetPodAllocatedGpuResources(gpuResourceName string, PodResources *podresourcesapi.GetPodResourcesResponse) []string {
	var gpuDevices []string
	for _, c := range PodResources.PodResources.Containers {
		// device plugin resouresouces
		for _, dev := range c.Devices {
			if dev.ResourceName != gpuResourceName {
				continue
			}

			// only one gpu resource
			gpuDevices = append(gpuDevices, dev.DeviceIds...)
		}
	}
	return gpuDevices
}

// LocalEndpoint returns the full path to a unix socket at the given endpoint
// which is in k8s.io/kubernetes/pkg/kubelet/util
func localEndpoint(path string) *url.URL {
	return &url.URL{
		Scheme: unixProtocol,
		Path:   path + ".sock",
	}
}

// GetResourceClient returns an instance of ResourceClient interface initialized with Pod resource information
func GetKubeletResourceClient(kubeletSocket string) (podresourcesapi.PodResourcesListerClient, *grpc.ClientConn, error) {
	kubeletSocketURL := localEndpoint(filepath.Join(defaultPodResourcesPath, defaultKubeletSocket))

	if kubeletSocket != "" {
		kubeletSocketURL = &url.URL{
			Scheme: unixProtocol,
			Path:   kubeletSocket,
		}
	}
	// If Kubelet resource API endpoint exist use that by default
	// Or else fallback with checkpoint file
	if !hasKubeletAPIEndpoint(kubeletSocketURL) {
		return nil, nil, fmt.Errorf("GetResourceClient: no Kubelet resource API endpoint found")
	}

	return getKubeletResourceClient(kubeletSocketURL)
}

func dial(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, unixProtocol, addr)
}

func getKubeletResourceClient(kubeletSocketURL *url.URL) (podresourcesapi.PodResourcesListerClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(kubeletSocketURL.Path, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dial),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaultPodResourcesMaxSize)))
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing socket %s: %v", kubeletSocketURL.Path, err)
	}
	return podresourcesapi.NewPodResourcesListerClient(conn), conn, nil
}

func hasKubeletAPIEndpoint(url *url.URL) bool {
	// Check for kubelet resource API socket file
	if _, err := os.Stat(url.Path); err != nil {
		return false
	}
	return true
}
