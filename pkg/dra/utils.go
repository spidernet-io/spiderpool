package dra

import (
	"os"
	"strings"

	"github.com/spidernet-io/spiderpool/pkg/constant"
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
