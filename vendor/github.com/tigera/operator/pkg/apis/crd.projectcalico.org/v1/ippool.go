// Copyright (c) 2020-2024 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KindIPPool     = "IPPool"
	KindIPPoolList = "IPPoolList"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPPool contains information about an IPPool resource.
type IPPool struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the IPPool.
	Spec IPPoolSpec `json:"spec,omitempty"`
}

// IPPoolSpec contains the specification for an IPPool resource.
type IPPoolSpec struct {
	// The pool CIDR.
	CIDR string `json:"cidr" validate:"net"`

	// Contains configuration for VXLAN tunneling for this pool. If not specified,
	// then this is defaulted to "Never" (i.e. VXLAN tunelling is disabled).
	VXLANMode VXLANMode `json:"vxlanMode,omitempty" validate:"omitempty,vxlanMode"`

	// Contains configuration for IPIP tunneling for this pool. If not specified,
	// then this is defaulted to "Never" (i.e. IPIP tunelling is disabled).
	IPIPMode IPIPMode `json:"ipipMode,omitempty" validate:"omitempty,ipIpMode"`

	// When nat-outgoing is true, packets sent from Calico networked containers in
	// this pool to destinations outside of this pool will be masqueraded.
	NATOutgoing bool `json:"natOutgoing,omitempty"`

	// When disabled is true, Calico IPAM will not assign addresses from this pool.
	Disabled bool `json:"disabled,omitempty"`

	// Disable exporting routes from this IP Pool's CIDR over BGP. [Default: false]
	DisableBGPExport bool `json:"disableBGPExport,omitempty" validate:"omitempty"`

	// The block size to use for IP address assignments from this pool. Defaults to 26 for IPv4 and 112 for IPv6.
	BlockSize int `json:"blockSize,omitempty"`

	// Allows IPPool to allocate for a specific node by label selector.
	NodeSelector string `json:"nodeSelector,omitempty" validate:"omitempty,selector"`

	// AWSSubnetID if specified Calico will attempt to ensure that IPs chosen from this IP pool are routed
	// to the corresponding node by adding one or more secondary ENIs to the node and explicitly assigning
	// the IP to one of the secondary ENIs.  Important: since subnets cannot cross availability zones,
	// it's important to use Kubernetes node selectors to avoid scheduling pods to one availability zone
	// using an IP pool that is backed by a subnet that belongs to another availability zone. If AWSSubnetID
	// is specified, then the CIDR of the IP pool must be contained within the specified AWS subnet.
	AWSSubnetID string `json:"awsSubnetID,omitempty" validate:"omitempty"`

	// AllowedUse controls what the IP pool will be used for.  If not specified or empty, defaults to
	// ["Tunnel", "Workload"] for back-compatibility
	AllowedUses []IPPoolAllowedUse `json:"allowedUses,omitempty" validate:"omitempty"`
}

type IPPoolAllowedUse string

const (
	IPPoolAllowedUseWorkload IPPoolAllowedUse = "Workload"
	IPPoolAllowedUseTunnel   IPPoolAllowedUse = "Tunnel"
)

type VXLANMode string

const (
	VXLANModeNever       VXLANMode = "Never"
	VXLANModeAlways      VXLANMode = "Always"
	VXLANModeCrossSubnet VXLANMode = "CrossSubnet"
)

type IPIPMode string

const (
	IPIPModeNever       IPIPMode = "Never"
	IPIPModeAlways      IPIPMode = "Always"
	IPIPModeCrossSubnet IPIPMode = "CrossSubnet"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPPoolList contains a list of IPPool resources.
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []IPPool `json:"items"`
}

// NewIPPool creates a new (zeroed) IPPool struct with the TypeMetadata initialised to the current
// version.
func NewIPPool() *IPPool {
	return &IPPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindIPPool,
			APIVersion: "crd.projectcalico.org/v1",
		},
	}
}

// NewIPPoolList creates a new (zeroed) IPPoolList struct with the TypeMetadata initialised to the current
// version.
func NewIPPoolList() *IPPoolList {
	return &IPPoolList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindIPPoolList,
			APIVersion: "crd.projectcalico.org/v1",
		},
	}
}
