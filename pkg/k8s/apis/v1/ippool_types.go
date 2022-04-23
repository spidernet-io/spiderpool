// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Route struct {
	// TODO

	// +kubebuilder:validation:Optional
	Destination string `json:"dst,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Gateway string `json:"gw,omitempty"`
}

// IPPoolSpec defines the desired state of IPPool.
type IPPoolSpec struct {

	// Specifies the IP version used by the IP pool.
	// Valid values are:
	// - "IPv4":
	// - "IPv6":

	// +kubebuilder:validation:Enum=IPv4;IPv6
	// +kubebuilder:validation:Required
	IPVersion string `json:"ipVersion"`

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

	// +kubebuilder:validation:Maximum=4095
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	Vlan *int32 `json:"vlan,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	NamesapceSelector *metav1.LabelSelector `json:"namesapceSelector,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`
}

// AllocatedIPs is a map of allocated IPs indexed by IP
type AllocatedIPs map[string]AllocatedIP

// AllocatedIP is an IP already has been allocated
type AllocatedIP struct {
	// TODO

	// +kubebuilder:validation:Optional
	Containerid string `json:"containerid,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	NIC string `json:"interface,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Node string `json:"node,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Pod string `json:"pod,omitempty"`
}

// IPPoolStatus defines the observed state of IPPool.
type IPPoolStatus struct {
	// TODO

	// +kubebuilder:validation:Optional
	AllocatedIPs AllocatedIPs `json:"allocatedIPs,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocateCount *int32 `json:"allocateCount,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	DeallocateCount *int32 `json:"deallocateCount,omitempty"`

	// TODO

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int32 `json:"totalIPCount,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="ippools",scope="Cluster",shortName={spl},singular="ippool"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// IPPool is the Schema for the ippools API.
type IPPool struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec IPPoolSpec `json:"spec,omitempty"`

	// +kubebuilder:validation:Optional
	Status IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPPoolList contains a list of IPPool.
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []IPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPPool{}, &IPPoolList{})
}
