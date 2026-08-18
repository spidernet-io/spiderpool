// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package client

// AllocateIPRequest represents the request body for the IaaS sub-ENI
// allocation API. Each SubEniRequests item describes one dual-stack sub-ENI
// to provision on a parent NIC.
type AllocateIPRequest struct {
	// PodName is optional
	PodName string `json:"podName,omitempty"`
	// PodNamespace is optional
	PodNamespace string `json:"podNamespace,omitempty"`
	// PodUID is optional
	PodUID string `json:"podUID,omitempty"`
	// NodeName is required
	NodeName string `json:"nodeName"`
	// SubEniRequests is required, at least 1 item
	SubEniRequests []SubEniRequest `json:"subEniRequests"`
}

// SubEniRequest is one dual-stack sub-ENI request: an IPv4/IPv6 address pair
// provisioned atomically on a single sub-network-interface. Subnet is the
// dual-stack cloud subnet shared by both address families.
type SubEniRequest struct {
	// ParentNicMac is required
	ParentNicMac string `json:"parentNicMac"`
	// Subnet is required
	Subnet string `json:"subnet"`
	// IPv4Address is required
	IPv4Address string `json:"ipv4Address"`
	// IPv6Address is required
	IPv6Address string `json:"ipv6Address"`
	// IPv4PoolName is the SpiderIPPool name the IPv4 address was allocated
	// from. Optional: the provider uses it to attribute the sub-ENI to a
	// global pool (ownership tagging + metadata flush) without a reverse
	// {subnet, ip} lookup. Absent for legacy callers.
	IPv4PoolName string `json:"ipv4PoolName,omitempty"`
	// IPv6PoolName is the SpiderIPPool name the IPv6 address was allocated
	// from. Optional, see IPv4PoolName.
	IPv6PoolName string `json:"ipv6PoolName,omitempty"`
}

// AllocateIPResponse represents the response from the IaaS sub-ENI
// allocation API
type AllocateIPResponse struct {
	// PodName from the response
	PodName string `json:"podName"`
	// PodNamespace from the response
	PodNamespace string `json:"podNamespace"`
	// NodeName from the response
	NodeName string `json:"nodeName"`
	// SubEniResponses contains the allocation results
	SubEniResponses []SubEniResult `json:"subEniResponses"`
}

// SubEniResult represents a single sub-ENI allocation result. MacAddress and
// VlanID are shared by both address families of the sub-ENI.
type SubEniResult struct {
	// ParentNicMac is the parent NIC MAC address
	ParentNicMac string `json:"parentNicMac"`
	// Subnet is the subnet CIDR
	Subnet string `json:"subnet"`
	// IPv4Address is the allocated IPv4 address
	IPv4Address string `json:"ipv4Address"`
	// IPv6Address is the allocated IPv6 address
	IPv6Address string `json:"ipv6Address"`
	// MacAddress is the MAC address of the sub-ENI
	MacAddress string `json:"macAddress"`
	// VlanID is the VLAN ID
	VlanID int64 `json:"vlanId"`
}

// ReleaseIPRequest represents the request body for IaaS IP release API.
// Releasing either address of a dual-stack sub-ENI deletes the whole
// sub-ENI on the cloud side, so one call per sub-ENI is sufficient.
type ReleaseIPRequest struct {
	// PodName is optional
	PodName string `json:"podName,omitempty"`
	// PodNamespace is optional
	PodNamespace string `json:"podNamespace,omitempty"`
	// PodUID is optional
	PodUID string `json:"podUID,omitempty"`
	// NodeName is required
	NodeName string `json:"nodeName"`
	// ParentNicMac is optional
	ParentNicMac string `json:"parentNicMac,omitempty"`
	// Subnet is required
	Subnet string `json:"subnet"`
	// IPAddress is the IP being released
	IPAddress string `json:"ipAddress"`
	// PoolName is the SpiderIPPool name the released IP belongs to.
	// Optional: same attribution purpose as SubEniRequest.IPv4PoolName.
	PoolName string `json:"poolName,omitempty"`
}
