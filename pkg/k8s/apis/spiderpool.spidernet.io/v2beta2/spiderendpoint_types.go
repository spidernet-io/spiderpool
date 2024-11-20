// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadEndpointStatus defines the observed state of SpiderEndpoint.
type WorkloadEndpointStatus struct {
	// +kubebuilder:validation:Required
	Current PodIPAllocation `json:"current"`

	// +kubebuilder:validation:Required
	OwnerControllerType string `json:"ownerControllerType"`

	// +kubebuilder:validation:Required
	OwnerControllerName string `json:"ownerControllerName"`
}

type PodIPAllocation struct {
	// +kubebuilder:validation:Required
	UID string `json:"uid"`

	// +kubebuilder:validation:Required
	Node string `json:"node"`

	// +kubebuilder:validation:Required
	IPs []IPAllocationDetail `json:"ips"`
}

type IPAllocationDetail struct {
	// +kubebuilder:validation:Required
	NIC string `json:"interface"`

	// +kubebuilder:validation:Optional
	IPv4 *string `json:"ipv4,omitempty"`

	// +kubebuilder:validation:Optional
	IPv6 *string `json:"ipv6,omitempty"`

	// +kubebuilder:validation:Optional
	IPv4Pool *string `json:"ipv4Pool,omitempty"`

	// +kubebuilder:validation:Optional
	IPv6Pool *string `json:"ipv6Pool,omitempty"`

	// DEPRECATED: Vlan is deprecated.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Maximum=4094
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	Vlan *int64 `json:"vlan,omitempty"`

	// +kubebuilder:validation:Optional
	IPv4Gateway *string `json:"ipv4Gateway,omitempty"`

	// +kubebuilder:validation:Optional
	IPv6Gateway *string `json:"ipv6Gateway,omitempty"`

	// +kubebuilder:validation:Optional
	CleanGateway *bool `json:"cleanGateway,omitempty"`

	// +kubebuilder:validation:Optional
	Routes []Route `json:"routes,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderendpoints",scope="Namespaced",shortName={se},singular="spiderendpoint"
// +kubebuilder:printcolumn:JSONPath=".status.current.ips[0].interface",description="interface",name="INTERFACE",type=string
// +kubebuilder:printcolumn:JSONPath=".status.current.ips[0].ipv4Pool",description="ipv4Pool",name="IPV4POOL",type=string
// +kubebuilder:printcolumn:JSONPath=".status.current.ips[0].ipv4",description="ipv4",name="IPV4",type=string
// +kubebuilder:printcolumn:JSONPath=".status.current.ips[0].ipv6Pool",description="ipv6Pool",name="IPV6POOL",type=string
// +kubebuilder:printcolumn:JSONPath=".status.current.ips[0].ipv6",description="ipv6",name="IPV6",type=string
// +kubebuilder:printcolumn:JSONPath=".status.current.node",description="node",name="NODE",type=string
// +kubebuilder:object:root=true

// Spiderndpoint is the Schema for the spiderendpoints API.
type SpiderEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status WorkloadEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderEndpointList contains a list of SpiderEndpoint.
type SpiderEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderEndpoint{}, &SpiderEndpointList{})
}
