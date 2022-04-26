// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadEndpointStatus defines the observed state of WorkloadEndpoint
type WorkloadEndpointStatus struct {
	// TODO

	// +kubebuilder:validation:Required
	Current PodIPAllocation `json:"current"`

	// TODO

	// +kubebuilder:validation:Optional
	History []PodIPAllocation `json:"history,omitempty"`
}

// TODO
type PodIPAllocation struct {
	// TODO

	// +kubebuilder:validation:Required
	Containerid string `json:"containerid"`

	// TODO

	// +kubebuilder:validation:Required
	Node string `json:"node"`

	// TODO

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	IPs []IPAllocationDetail `json:"ips"`

	// TODO

	// +kubebuilder:validation:Required
	CreationTime metav1.Time `json:"creationTime"`
}

// TODO
type IPAllocationDetail struct {
	// TODO

	// +kubebuilder:validation:Required
	NIC string `json:"interface"`

	// TODO

	// +kubebuilder:validation:Optional
	IPv4 *string `json:"ipv4,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	IPv6 *string `json:"ipv6,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	IPv4Pool *string `json:"ipv4Pool,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	IPv6Pool *string `json:"ipv6Pool,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Vlan *Vlan `json:"vlan,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Route *Route `json:"route,omitempty"`

	// TODO

	// +kubebuilder:validation:Optional
	Gateway *string `json:"gateway,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="workloadendpoints",scope="Namespaced",shortName={swe},singular="workloadendpoint"
// +kubebuilder:printcolumn:JSONPath=".status.ips.interface",description="interface",name="INTERFACE",type=string
// +kubebuilder:printcolumn:JSONPath=".status.ips.ipv4Pool",description="ipv4Pool",name="IPV4POOL",type=string
// +kubebuilder:printcolumn:JSONPath=".status.ips.ipv4",description="ipv4",name="IPV4",type=string
// +kubebuilder:printcolumn:JSONPath=".status.ips.ipv6Pool",description="ipv6Pool",name="IPV6POOL",type=string
// +kubebuilder:printcolumn:JSONPath=".status.ips.ipv6",description="ipv6",name="IPV6",type=string
// +kubebuilder:printcolumn:JSONPath=".status.node",description="node",name="NODE",type=string
// +kubebuilder:printcolumn:JSONPath=".status.current.creationTime",description="creationTime",name="CREATETION TIME",type=date
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WorkloadEndpoint is the Schema for the workloadendpoints API
type WorkloadEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status WorkloadEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadEndpointList contains a list of WorkloadEndpoint
type WorkloadEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []WorkloadEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadEndpoint{}, &WorkloadEndpointList{})
}
