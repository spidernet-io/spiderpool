// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

/**
* Copyright (c) 2017 Intel Corporation
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package multuscniconfig

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	ifacercmd "github.com/spidernet-io/spiderpool/cmd/ifacer/cmd"
	spiderpoolcmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

const (
	MacVlanType = "macvlan"
	IpVlanType  = "ipvlan"
	SriovType   = "sriov"
	CustomType  = "custom"
)

type MacvlanNetConf struct {
	Type   string                   `json:"type"`
	IPAM   spiderpoolcmd.IPAMConfig `json:"ipam"`
	Master string                   `json:"master"`
	Mode   string                   `json:"mode"`
}

type IPvlanNetConf struct {
	Type   string                   `json:"type"`
	IPAM   spiderpoolcmd.IPAMConfig `json:"ipam"`
	Master string                   `json:"master"`
}

type SRIOVNetConf struct {
	Type         string                   `json:"type"`
	ResourceName string                   `json:"resourceName"` // required
	IPAM         spiderpoolcmd.IPAMConfig `json:"ipam"`
	Vlan         *int32                   `json:"vlan,omitempty"`
	DeviceID     string                   `json:"deviceID,omitempty"`
}

type IfacerNetConf = ifacercmd.Ifacer

func ParsePodNetworkAnnotation(podNetworks, defaultNamespace string) ([]*netv1.NetworkSelectionElement, error) {
	var networks []*netv1.NetworkSelectionElement

	if podNetworks == "" {
		return nil, fmt.Errorf("parsePodNetworkAnnotation: %s, %s", podNetworks, defaultNamespace)
	}

	if strings.HasPrefix(podNetworks, "[{\"") {
		if err := json.Unmarshal([]byte(podNetworks), &networks); err != nil {
			return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to parse pod Network Attachment Selection Annotation JSON format: %v", err)
		}
	} else {
		// Comma-delimited list of network attachment object names
		for _, item := range strings.Split(podNetworks, ",") {
			// Remove leading and trailing whitespace.
			item = strings.TrimSpace(item)

			// Parse network name (i.e. <namespace>/<network name>@<ifname>)
			netNsName, networkName, netIfName, err := ParsePodNetworkObjectName(item)
			if err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: %v", err)
			}

			networks = append(networks, &netv1.NetworkSelectionElement{
				Name:             networkName,
				Namespace:        netNsName,
				InterfaceRequest: netIfName,
			})
		}
	}

	for _, n := range networks {
		if n.Namespace == "" {
			n.Namespace = defaultNamespace
		}
		if n.MacRequest != "" {
			// validate MAC address
			if _, err := net.ParseMAC(n.MacRequest); err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to mac: %v", err)
			}
		}
		if n.InfinibandGUIDRequest != "" {
			// validate GUID address
			if _, err := net.ParseMAC(n.InfinibandGUIDRequest); err != nil {
				return nil, fmt.Errorf("parsePodNetworkAnnotation: failed to validate infiniband GUID: %v", err)
			}
		}
		if n.IPRequest != nil {
			for _, ip := range n.IPRequest {
				// validate IP address
				if strings.Contains(ip, "/") {
					if _, _, err := net.ParseCIDR(ip); err != nil {
						return nil, fmt.Errorf("failed to parse CIDR %q: %v", ip, err)
					}
				} else if net.ParseIP(ip) == nil {
					return nil, fmt.Errorf("failed to parse IP address %q", ip)
				}
			}
		}

		// compatibility pre v3.2, will be removed in v4.0
		// if n.DeprecatedInterfaceRequest != "" && n.InterfaceRequest == "" {
		//	n.InterfaceRequest = n.DeprecatedInterfaceRequest
		// }
	}

	return networks, nil
}

func ParsePodNetworkObjectName(podnetwork string) (string, string, string, error) {
	var netNsName string
	var netIfName string
	var networkName string

	slashItems := strings.Split(podnetwork, "/")
	if len(slashItems) == 2 {
		netNsName = strings.TrimSpace(slashItems[0])
		networkName = slashItems[1]
	} else if len(slashItems) == 1 {
		networkName = slashItems[0]
	} else {
		return "", "", "", fmt.Errorf("parsePodNetworkObjectName: Invalid network object (failed at '/')")
	}

	atItems := strings.Split(networkName, "@")
	networkName = strings.TrimSpace(atItems[0])
	if len(atItems) == 2 {
		netIfName = strings.TrimSpace(atItems[1])
	} else if len(atItems) != 1 {
		return "", "", "", fmt.Errorf("parsePodNetworkObjectName: Invalid network object (failed at '@')")
	}

	// Check and see if each item matches the specification for valid attachment name.
	// "Valid attachment names must be comprised of units of the DNS-1123 label format"
	// [a-z0-9]([-a-z0-9]*[a-z0-9])?
	// And we allow at (@), and forward slash (/) (units separated by commas)
	// It must start and end alphanumerically.
	regexpStr := "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	compile, err := regexp.Compile(regexpStr)
	if nil != err {
		return "", "", "", fmt.Errorf("failed to parse regexp expression for %s, error: %w", regexpStr, err)
	}

	allItems := []string{netNsName, networkName, netIfName}
	for i := range allItems {
		matched := compile.MatchString(allItems[i])
		if !matched && len([]rune(allItems[i])) > 0 {
			return "", "", "", fmt.Errorf(fmt.Sprintf("parsePodNetworkObjectName: Failed to parse: one or more items did not match comma-delimited format (must consist of lower case alphanumeric characters). Must start and end with an alphanumeric character), mismatch @ '%v'", allItems[i]))
		}
	}

	return netNsName, networkName, netIfName, nil
}
