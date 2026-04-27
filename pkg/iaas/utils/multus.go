// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/multuscniconfig"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
)

// NetworkSelectionElement represents a network selection element
type NetworkSelectionElement struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Interface string `json:"interface,omitempty"`
}

// GetParentNicMac gets the parent NIC MAC address for a given interface
// This is a simplified implementation for Phase 2
// Full implementation should:
// 1. Parse Pod annotation to get SpiderMultusConfig
// 2. Check if it's vlan CNI type
// 3. Get master interface name from config
// 4. Use netlink to get MAC address
func GetParentNicMac(ctx context.Context, pod *corev1.Pod, ifName string) (string, error) {
	// For now, get the MAC from the host network interface
	// In a full implementation, this would:
	// 1. Parse "k8s.v1.cni.cncf.io/networks" annotation
	// 2. Find the matching SpiderMultusConfig
	// 3. Get master interface from config
	// 4. Return master MAC

	// Get the link by name
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return "", fmt.Errorf("failed to get link %s: %w", ifName, err)
	}

	return link.Attrs().HardwareAddr.String(), nil
}

// MultusNetworkInfo holds the multus network configuration for a specific NIC
type MultusNetworkInfo struct {
	// Namespace of the multus network attachment definition
	Namespace string
	// Name of the multus network attachment definition
	Name string
}

// GetMultusNetworkForNIC retrieves the multus network configuration for a given NIC.
// It parses the pod's Multus annotations (both default network and additional networks)
// and returns the matching network attachment definition info.
//
// Parameters:
//   - pod: the pod containing Multus annotations
//   - nic: the NIC name (e.g., "eth0", "net1", or "1" for index-based lookup)
//   - agentNamespace: the namespace where multus resources are defined (for default network)
//   - clusterNetwork: optional cluster default network configuration
//
// Returns the MultusNetworkInfo containing namespace and name of the network attachment definition.
// This function is based on the logic from pkg/ipam/allocate.go and can be used by both
// IPAM and IaaS modules.
func GetMultusNetworkForNIC(pod *corev1.Pod, nic, agentNamespace string, clusterNetwork *string) (*MultusNetworkInfo, error) {
	podAnno := pod.GetAnnotations()

	// Check for default NIC (eth0 or index 0)
	if nic == constant.ClusterDefaultInterfaceName || nic == strconv.Itoa(0) {
		return getDefaultMultusNetwork(podAnno, agentNamespace, clusterNetwork)
	}

	// For additional NICs (net1, net2, etc.), parse the networks annotation
	return getAdditionalMultusNetwork(podAnno, pod.Namespace, nic)
}

// getDefaultMultusNetwork retrieves the default multus network configuration.
func getDefaultMultusNetwork(podAnno map[string]string, agentNamespace string, clusterNetwork *string) (*MultusNetworkInfo, error) {
	// Check for default network annotation
	defaultMultusObj := podAnno[constant.MultusDefaultNetAnnot]
	if len(defaultMultusObj) == 0 {
		if clusterNetwork == nil {
			return nil, fmt.Errorf("no default multus network configured")
		}
		defaultMultusObj = *clusterNetwork
	}

	// Parse the annotation
	networks, err := multuscniconfig.ParsePodNetworkAnnotation(defaultMultusObj, agentNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default network annotation: %w", err)
	}
	if len(networks) == 0 {
		return nil, fmt.Errorf("empty default network annotation")
	}

	// Use the first network as default
	ns := networks[0].Namespace
	if ns == "" {
		ns = agentNamespace
	}

	return &MultusNetworkInfo{
		Namespace: ns,
		Name:      networks[0].Name,
	}, nil
}

// getAdditionalMultusNetwork retrieves the multus network for an additional NIC.
func getAdditionalMultusNetwork(podAnno map[string]string, podNamespace, nic string) (*MultusNetworkInfo, error) {
	annotation := podAnno[constant.MultusNetworkAttachmentAnnot]
	if annotation == "" {
		return nil, fmt.Errorf("no multus network attachment annotation found")
	}

	networks, err := multuscniconfig.ParsePodNetworkAnnotation(annotation, podNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to parse network attachment annotation: %w", err)
	}

	// Find matching network by NIC name or index
	for idx, network := range networks {
		// Default interface name if not specified
		ifName := network.InterfaceRequest
		if ifName == "" {
			ifName = fmt.Sprintf("net%d", idx+1)
		}

		// Match by interface name or index
		if nic == ifName || nic == strconv.Itoa(idx+1) {
			ns := network.Namespace
			if ns == "" {
				ns = podNamespace
			}
			return &MultusNetworkInfo{
				Namespace: ns,
				Name:      network.Name,
			}, nil
		}
	}

	return nil, fmt.Errorf("no matching multus network found for NIC %s", nic)
}

// ParseMacAddress parses a MAC address string
func ParseMacAddress(mac string) (net.HardwareAddr, error) {
	return net.ParseMAC(mac)
}
