// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	corev1 "k8s.io/api/core/v1"
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

func ExtractK8sCIDRFromKubeadmConfigMap(cm *corev1.ConfigMap) ([]string, []string, error) {
	if cm == nil {
		return nil, nil, fmt.Errorf("kubeadm configmap is unexpected to nil")
	}
	var podCIDR, serviceCIDR []string

	clusterConfig, exists := cm.Data["ClusterConfiguration"]
	if !exists {
		return podCIDR, serviceCIDR, fmt.Errorf("unable to get kubeadm configmap ClusterConfiguration")
	}

	podReg := regexp.MustCompile(`podSubnet:\s*(\S+)`)
	serviceReg := regexp.MustCompile(`serviceSubnet:\s*(\S+)`)

	podSubnets := podReg.FindStringSubmatch(clusterConfig)
	serviceSubnets := serviceReg.FindStringSubmatch(clusterConfig)

	if len(podSubnets) > 1 {
		for _, cidr := range strings.Split(podSubnets[1], ",") {
			cidr = strings.TrimSpace(cidr)
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			podCIDR = append(podCIDR, cidr)
		}
	}

	if len(serviceSubnets) > 1 {
		for _, cidr := range strings.Split(serviceSubnets[1], ",") {
			cidr = strings.TrimSpace(cidr)
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			serviceCIDR = append(serviceCIDR, cidr)
		}
	}

	return podCIDR, serviceCIDR, nil
}

func ExtractK8sCIDRFromKCMPod(kcm *corev1.Pod) ([]string, []string) {
	var podCIDR, serviceCIDR []string

	podReg := regexp.MustCompile(`--cluster-cidr=(.*)`)
	serviceReg := regexp.MustCompile(`--service-cluster-ip-range=(.*)`)

	var podSubnets, serviceSubnets []string
	findSubnets := func(l string) {
		if len(podSubnets) == 0 {
			podSubnets = podReg.FindStringSubmatch(l)
		}
		if len(serviceSubnets) == 0 {
			serviceSubnets = serviceReg.FindStringSubmatch(l)
		}
	}

	for _, l := range kcm.Spec.Containers[0].Command {
		findSubnets(l)
		if len(podSubnets) != 0 && len(serviceSubnets) != 0 {
			break
		}
	}

	if len(podSubnets) == 0 || len(serviceSubnets) == 0 {
		for _, l := range kcm.Spec.Containers[0].Args {
			findSubnets(l)
			if len(podSubnets) != 0 && len(serviceSubnets) != 0 {
				break
			}
		}
	}

	if len(podSubnets) != 0 {
		for _, cidr := range strings.Split(podSubnets[1], ",") {
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			podCIDR = append(podCIDR, cidr)
		}
	}

	if len(serviceSubnets) != 0 {
		for _, cidr := range strings.Split(serviceSubnets[1], ",") {
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			serviceCIDR = append(serviceCIDR, cidr)
		}
	}

	return podCIDR, serviceCIDR
}

func AbsInt(a, b int) int {
	if a > b {
		return a - b
	}

	return b - a
}

// GetNodeName returns the current node name
func GetNodeName() string {
	return os.Getenv(constant.ENV_SPIDERPOOL_NODENAME)
}

func GetAgentNamespace() string {
	if os.Getenv(constant.ENV_SPIDERPOOL_AGENT_NAMESPACE) == "" {
		return constant.Spiderpool
	}
	return os.Getenv(constant.ENV_SPIDERPOOL_AGENT_NAMESPACE)
}
