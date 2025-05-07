package dra

import (
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

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

// NormalizedDNS1123Label normalizes the interface name to a valid DNS1123 label
func NormalizedDNS1123Label(iface string) string {
	// Convert to lowercase
	normalized := strings.ToLower(iface)
	// Replace invalid chars with hyphen
	reg := regexp.MustCompile("[^a-z0-9-]")
	normalized = reg.ReplaceAllString(normalized, "-")
	// Remove leading and trailing hyphens
	normalized = strings.Trim(normalized, "-")
	// Replace multiple consecutive hyphens with a single one
	reg = regexp.MustCompile("-+")
	normalized = reg.ReplaceAllString(normalized, "-")

	// If the string is empty after normalization, use a default name
	if normalized == "" {
		normalized = "iface"
	}
	// If it starts with a number, prefix it
	if unicode.IsDigit(rune(normalized[0])) {
		normalized = "iface-" + normalized
	}
	return normalized
}
