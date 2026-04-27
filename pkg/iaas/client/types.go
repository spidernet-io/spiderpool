// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package client

// AllocateIPRequest represents the request body for IaaS IP allocation API
type AllocateIPRequest struct {
	// PodName is optional
	PodName string `json:"podName,omitempty"`
	// PodNamespace is optional
	PodNamespace string `json:"podNamespace,omitempty"`
	// PodUID is optional
	PodUID string `json:"podUID,omitempty"`
	// NodeName is required
	NodeName string `json:"nodeName"`
	// IaaSIPsAllocationRequest is required, at least 1 item
	IaaSIPsAllocationRequest []IaaSIPAllocationItem `json:"iaasIPsAllocationRequest"`
}

// IaaSIPAllocationItem represents a single IP allocation request item
type IaaSIPAllocationItem struct {
	// IPAddress is required
	IPAddress string `json:"ipAddress"`
	// Subnet is required
	Subnet string `json:"subnet"`
	// ParentNicMac is required
	ParentNicMac string `json:"parentNicMac"`
}

// AllocateIPResponse represents the response from IaaS IP allocation API
type AllocateIPResponse struct {
	// PodName from the response
	PodName string `json:"podName"`
	// PodNamespace from the response
	PodNamespace string `json:"podNamespace"`
	// NodeName from the response
	NodeName string `json:"nodeName"`
	// IaaSIPsAllocationResponse contains the allocation results
	IaaSIPsAllocationResponse []IaaSIPAllocationResult `json:"iaasIPsAllocationResponse"`
}

// IaaSIPAllocationResult represents a single IP allocation result
type IaaSIPAllocationResult struct {
	// ParentNicMac is the parent NIC MAC address
	ParentNicMac string `json:"parentNicMac"`
	// Subnet is the subnet CIDR
	Subnet string `json:"subnet"`
	// IPAddress is the allocated IP address
	IPAddress string `json:"ipAddress"`
	// MacAddress is the MAC address for the allocated IP
	MacAddress string `json:"macAddress"`
	// VlanID is the VLAN ID
	VlanID int64 `json:"vlanId"`
}

// ReleaseIPRequest represents the request body for IaaS IP release API
type ReleaseIPsRequest struct {
	// PodName is optional
	PodName string `json:"podName,omitempty"`
	// PodNamespace is optional
	PodNamespace string `json:"podNamespace,omitempty"`
	// PodUID is optional
	PodUID string `json:"podUID,omitempty"`
	// NodeName is required
	NodeName string `json:"nodeName"`
	// IPAddresses are the IPs being released
	IPAddresses []string `json:"ipAddresses"`
}

type ReleaseIPRequest struct {
	// NodeName is required
	NodeName string `json:"nodeName"`
	// IPAddress is the IP being released
	IPAddress    string `json:"ipAddress"`
	Subnet       string `json:"subnet"`
	ParentNicMac string `json:"parentNicMac"`
}
