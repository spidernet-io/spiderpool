// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package cmd

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/constant"
)

const (
	CniVersion030 = "0.3.0"
	CniVersion031 = "0.3.1"
	CniVersion040 = "0.4.0"
)

// SupportCNIVersions indicate the CNI version that spiderpool support.
var SupportCNIVersions = []string{CniVersion030, CniVersion031, CniVersion040}

const DefaultLogLevelStr = constant.LogInfoLevelStr

// K8sArgs is the valid CNI_ARGS used for Kubernetes
type K8sArgs struct {
	types.CommonArgs
	IP                         net.IP
	K8S_POD_NAME               types.UnmarshallableString //revive:disable-line
	K8S_POD_NAMESPACE          types.UnmarshallableString //revive:disable-line
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString //revive:disable-line
	K8S_POD_UID                types.UnmarshallableString //revive:disable-line
}

// NetConf for cni config file written in json
type NetConf struct {
	Name       string     `json:"name"`
	CNIVersion string     `json:"cniVersion"`
	IPAM       IPAMConfig `json:"ipam"`
}

// IPAMConfig is a custom IPAM struct, you can check reference details: https://www.cni.dev/docs/spec/#plugin-configuration-objects
type IPAMConfig struct {
	Type string `json:"type"`

	LogLevel        string `json:"log_level"`
	LogFilePath     string `json:"log_file_path"`
	LogFileMaxSize  int    `json:"log_file_max_size"`
	LogFileMaxAge   int    `json:"log_file_max_age"`
	LogFileMaxCount int    `json:"log_file_max_count"`

	DefaultIPv4IPPool []string `json:"default_ipv4_ippool"`
	DefaultIPv6IPPool []string `json:"default_ipv6_ippool"`

	IpamUnixSocketPath string `json:"ipam_unix_socket_path"`
}

// LoadNetConf converts inputs (i.e. stdin) to NetConf
func LoadNetConf(argsStdin []byte) (*NetConf, error) {
	netConf := &NetConf{}

	err := json.Unmarshal(argsStdin, netConf)
	if nil != err {
		return nil, fmt.Errorf("Unable to parse CNI configuration \"%s\": %s", argsStdin, err)
	}

	if netConf.IPAM.LogLevel == "" {
		netConf.IPAM.LogLevel = DefaultLogLevelStr
	}

	if netConf.IPAM.IpamUnixSocketPath == "" {
		netConf.IPAM.IpamUnixSocketPath = constant.DefaultIPAMUnixSocketPath
	}

	for _, vers := range SupportCNIVersions {
		if netConf.CNIVersion == vers {
			return netConf, nil
		}
	}

	return nil, fmt.Errorf("Error: Mismatch the given CNI Version: %s, spiderpool supports CNI version %#v", netConf.CNIVersion, SupportCNIVersions)
}
