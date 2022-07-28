// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPPoolSpec defines the desired state of IPPool
type IPPoolSpec struct {
	// +kubebuilder:validation:Enum=4;6
	// +kubebuilder:validation:Optional
	IPVersion *int64 `json:"ipVersion,omitempty"`

	// +kubebuilder:validation:Required
	Subnet string `json:"subnet"`

	// +kubebuilder:validation:Required
	IPs []string `json:"ips"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Disable *bool `json:"disable,omitempty"`

	// +kubebuilder:validation:Optional
	ExcludeIPs []string `json:"excludeIPs,omitempty"`

	// +kubebuilder:validation:Optional
	Gateway *string `json:"gateway,omitempty"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Maximum=4095
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	Vlan *int64 `json:"vlan,omitempty"`

	// +kubebuilder:validation:Optional
	Routes []Route `json:"routes,omitempty"`

	// +kubebuilder:validation:Optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	// +kubebuilder:validation:Optional
	NamesapceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// +kubebuilder:validation:Optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

type Route struct {
	// +kubebuilder:validation:Required
	Dst string `json:"dst"`

	// +kubebuilder:validation:Required
	Gw string `json:"gw"`
}

// IPPoolStatus defines the observed state of IPPool
type IPPoolStatus struct {
	// +kubebuilder:validation:Optional
	AllocatedIPs PoolIPAllocations `json:"allocatedIPs,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int64 `json:"totalIPCount,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocatedIPCount *int64 `json:"allocatedIPCount,omitempty"`
}

// PoolIPAllocations is a map of allocated IPs indexed by IP
type PoolIPAllocations map[string]PoolIPAllocation

// PoolIPAllocation is an IP already has been allocated
type PoolIPAllocation struct {
	// +kubebuilder:validation:Required
	ContainerID string `json:"containerID"`

	// +kubebuilder:validation:Required
	NIC string `json:"interface"`

	// +kubebuilder:validation:Required
	Node string `json:"node"`

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// +kubebuilder:validation:Required
	Pod string `json:"pod"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderippools",scope="Cluster",shortName={sp},singular="spiderippool"
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
