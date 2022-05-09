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

const DefaultLogLevelStr = "info"

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
	types.NetConf

	LogLevel        string `json:"log_level"`
	LogFilePath     string `json:"log_file_path"`
	LogFileMaxSize  int    `json:"log_file_max_size"`
	LogFileMaxAge   int    `json:"log_file_max_age"`
	LogFileMaxCount int    `json:"log_file_max_count"`

	IpamUnixSocketPath string `json:"ipam_unix_socket_path"`
}

// LoadNetConf converts inputs (i.e. stdin) to NetConf
func LoadNetConf(argsStdin []byte) (*NetConf, error) {
	netConf := &NetConf{}

	err := json.Unmarshal(argsStdin, netConf)
	if nil != err {
		return nil, fmt.Errorf("Unable to parse CNI configuration \"%s\": %s", argsStdin, err)
	}

	if netConf.LogLevel == "" {
		netConf.LogLevel = DefaultLogLevelStr
	}

	if netConf.IpamUnixSocketPath == "" {
		netConf.IpamUnixSocketPath = constant.DefaultIPAMUnixSocketPath
	}

	return netConf, nil
}
