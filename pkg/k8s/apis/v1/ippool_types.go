// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPPoolSpec defines the desired state of IPPool
type IPPoolSpec struct {

	// Specifies the IP version used by the IP pool
	// Valid values are:
	// - "IPv4":
	// - "IPv6":

	// +kubebuilder:validation:Required
	IPVersion IPVersion `json:"ipVersion"`

	// TODO

	// +kubebuilder:validation:Required
	Subnet string `json:"subnet"`

	// TODO

	// +kubebuilder:validation:Required
	IPs []string `json:"ips"`

	// TODO

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Disable *bool `json:"disable,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	ExcludeIPs []string `json:"excludeIPs,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Gateway *string `json:"gateway,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Routes []Route `json:"routes,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Vlan *Vlan `json:"vlan,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	NamesapceSelector *metav1.LabelSelector `json:"namesapceSelector,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

// +kubebuilder:validation:Enum=IPv4;IPv6
type IPVersion string

const (
	// TODO
	IPv4 IPVersion = "IPv4"

	// TODO
	IPv6 IPVersion = "IPv6"
)

// +kubebuilder:validation:Maximum=4095
// +kubebuilder:validation:Minimum=0
type Vlan int32

type Route struct {
	// TODO

	// +kubebuilder:validation:Required
	Dst string `json:"dst"`

	// TODO

	// +kubebuilder:validation:Optional
	Gw *string `json:"gw,omitempty"`
}

// IPPoolStatus defines the observed state of IPPool
type IPPoolStatus struct {
	// TODO

	// +kubebuilder:validation:Optional
	AllocatedIPs PoolIPAllocations `json:"allocatedIPs,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocateCount *int64 `json:"allocateCount,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	DeallocateCount *int64 `json:"deallocateCount,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int32 `json:"totalIPCount,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocatedIPCount *int32 `json:"allocatedIPCount,omitempty"`
}

// PoolIPAllocations is a map of allocated IPs indexed by IP
type PoolIPAllocations map[string]PoolIPAllocation

// PoolIPAllocation is an IP already has been allocated
type PoolIPAllocation struct {
	// TODO

	// +kubebuilder:validation:Required
	Containerid string `json:"containerid"`

	// TODO

	// +kubebuilder:validation:Required
	NIC string `json:"interface"`

	// TODO

	// +kubebuilder:validation:Required
	Node string `json:"node"`

	// TODO

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// TODO

	// +kubebuilder:validation:Required
	Pod string `json:"pod"`
}

// +kubebuilder:resource:categories={spiderpool},path="ippools",scope="Cluster",shortName={spl},singular="ippool"
// +kubebuilder:printcolumn:JSONPath=".spec.ipVersion",description="ipVersion",name="VERSION",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.subnet",description="subnet",name="SUBNET",type=string
// +kubebuilder:printcolumn:JSONPath=".status.allocatedIPCount",description="allocatedIPCount",name="ALLOCATED-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.totalIPCount",description="totalIPCount",name="TOTAL-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".spec.disable",description="disable",name="DISABLE",type=boolean
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// IPPool is the Schema for the ippools API
type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPPoolList contains a list of IPPool
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []IPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPPool{}, &IPPoolList{})
}
