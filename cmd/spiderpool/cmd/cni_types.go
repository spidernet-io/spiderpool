// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var BinNamePlugin = filepath.Base(os.Args[0])

var (
	ErrAgentHealthCheck = fmt.Errorf("unhealthy spiderpool-agent backend")
	ErrPostIPAM         = fmt.Errorf("spiderpool IP allocation error")
	ErrDeleteIPAM       = fmt.Errorf("spiderpool IP release error")
)

const (
	CniVersion030 = "0.3.0"
	CniVersion031 = "0.3.1"
	CniVersion040 = "0.4.0"
	CniVersion100 = "1.0.0"
)

// SupportCNIVersions indicate the CNI version that spiderpool support.
var SupportCNIVersions = []string{CniVersion030, CniVersion031, CniVersion040, CniVersion100}

const DefaultLogLevelStr = logutils.LogDebugLevelStr

// K8sArgs is the valid CNI_ARGS used for Kubernetes.
type K8sArgs struct {
	types.CommonArgs
	IP                         net.IP
	K8S_POD_NAME               types.UnmarshallableString //revive:disable-line
	K8S_POD_NAMESPACE          types.UnmarshallableString //revive:disable-line
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString //revive:disable-line
	K8S_POD_UID                types.UnmarshallableString //revive:disable-line
}

// NetConf is the structure of CNI network configuration.
type NetConf struct {
	Name       string     `json:"name"`
	CNIVersion string     `json:"cniVersion"`
	IPAM       IPAMConfig `json:"ipam"`
}

// IPAMConfig is a custom IPAM struct.
// Reference: https://www.cni.dev/docs/spec/#plugin-configuration-objects
type IPAMConfig struct {
	Type string `json:"type"`

	LogLevel        string `json:"log_level,omitempty"`
	LogFilePath     string `json:"log_file_path,omitempty"`
	LogFileMaxSize  int    `json:"log_file_max_size,omitempty"`
	LogFileMaxAge   int    `json:"log_file_max_age,omitempty"`
	LogFileMaxCount int    `json:"log_file_max_count,omitempty"`

	DefaultIPv4IPPool []string `json:"default_ipv4_ippool,omitempty"`
	DefaultIPv6IPPool []string `json:"default_ipv6_ippool,omitempty"`
	CleanGateway      bool     `json:"clean_gateway,omitempty"`
	MatchMasterSubnet bool     `json:"match_master_subnet,omitempty"`

	IPAMUnixSocketPath string `json:"ipam_unix_socket_path,omitempty"`
}

// LoadNetConf converts input (i.e. stdin) to NetConf.
func LoadNetConf(argsStdin []byte) (*NetConf, error) {
	netConf := &NetConf{}

	err := json.Unmarshal(argsStdin, netConf)
	if nil != err {
		return nil, fmt.Errorf("failed to parse CNI network configuration: %w", err)
	}

	if netConf.IPAM.LogLevel == "" {
		netConf.IPAM.LogLevel = DefaultLogLevelStr
	}

	if netConf.IPAM.IPAMUnixSocketPath == "" {
		netConf.IPAM.IPAMUnixSocketPath = constant.DefaultIPAMUnixSocketPath
	}

	for _, vers := range SupportCNIVersions {
		if netConf.CNIVersion == vers {
			return netConf, nil
		}
	}

	return nil, fmt.Errorf("unsupported specified CNI version %s, the CNI versions supported by Spiderpool: %v", netConf.CNIVersion, SupportCNIVersions)
}
