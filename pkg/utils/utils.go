// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/containernetworking/cni/libcni"
)

// GetDefaultCNIConfPath according to the provided CNI file path (default is /etc/cni/net.d),
// return the first CNI configuration file path under this path.
func GetDefaultCNIConfPath(cniDir string) (string, error) {
	return findDefaultCNIConf(cniDir)
}

// GetDefaultCniName according to the provided CNI file path (default is /etc/cni/net.d),
// the first CNI configuration file under this path is parsed and its name is returned
func GetDefaultCniName(cniDir string) (string, error) {
	cniPath, err := findDefaultCNIConf(cniDir)
	if err != nil {
		return "", err
	}

	if cniPath != "" {
		return fetchCniNameFromPath(cniPath)
	}
	return "", nil
}

func findDefaultCNIConf(cniDir string) (string, error) {
	cnifiles, err := libcni.ConfFiles(cniDir, []string{".conf", ".conflist"})
	if err != nil {
		return "", fmt.Errorf("failed to load cni files in %s: %v", cniDir, err)
	}

	var cniPluginConfigs []string
	for _, file := range cnifiles {
		if strings.Contains(file, "00-multus") {
			continue
		}
		cniPluginConfigs = append(cniPluginConfigs, file)
	}

	if len(cniPluginConfigs) == 0 {
		return "", nil
	}
	sort.Strings(cniPluginConfigs)

	return cniPluginConfigs[0], nil
}

func fetchCniNameFromPath(cniPath string) (string, error) {
	if strings.HasSuffix(cniPath, ".conflist") {
		confList, err := libcni.ConfListFromFile(cniPath)
		if err != nil {
			return "", fmt.Errorf("error loading CNI conflist file %s: %v", cniPath, err)
		}
		return confList.Name, nil
	}

	conf, err := libcni.ConfFromFile(cniPath)
	if err != nil {
		return "", fmt.Errorf("error loading CNI config file %s: %v", cniPath, err)
	}
	return conf.Network.Name, nil
}
