package dra

import (
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

const (
	defaultKubeletSocket       = "kubelet" // which is defined in k8s.io/kubernetes/pkg/kubelet/apis/podresources
	kubeletConnectionTimeout   = 10 * time.Second
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	defaultPodResourcesPath    = "/var/lib/kubelet/pod-resources"
	unixProtocol               = "unix"
)

// GetPciAddressPrefix returns the prefix of a PCI address
// [domain]:[bus]:[device].[function] -> [domain]:[bus]
// e.g. 0000:af:00.1 -> 0000:af
func GetPciAddressPrefix(pciAddress string) string {
	parts := strings.Split(pciAddress, ":")
	if len(parts) == 3 {
		return parts[0] + ":" + parts[1]
	}
	return ""
}

// GetNodeName returns the current node name
func GetNodeName() string {
	return os.Getenv(constant.ENV_SPIDERPOOL_NODENAME)
}

func GetAgentNamespace() string {
	return os.Getenv(constant.ENV_SPIDERPOOL_AGENT_NAMESPACE)
}

// LocalEndpoint returns the full path to a unix socket at the given endpoint
// which is in k8s.io/kubernetes/pkg/kubelet/util
func localEndpoint(path string) *url.URL {
	return &url.URL{
		Scheme: unixProtocol,
		Path:   path + ".sock",
	}
}
