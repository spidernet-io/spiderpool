// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeSpec defines the observed state of SpiderNode.
type NodeSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Optional
	PreAllocated *int64 `json:"pre-allocated,omitempty"`

	// could be allocate ips in this node, also include had allocatd.
	// +kubebuilder:validation:Optional
	Pools map[string][]string `json:"pools,omitempty"`
}

// NodeStatus defines the desired state of SpiderNode.
type NodeStatus struct {
	// +kubebuilder:validation:Optional
	Pools map[string][]string `json:"pools,omitempty"`

	// +kubebuilder:validation:Optional
	Enis map[string]Eni `json:"enis,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int64 `json:"totalIPCount,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocatedIPCount *int64 `json:"allocatedIPCount,omitempty"`
}

type IpItem struct {
	// +kubebuilder:validation:Required
	Ip string `json:"ip"`

	// +kubebuilder:validation:Optional
	Id string `json:"id,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ready;release
	Status string `json:"status"`
}

type Eni struct {
	// +kubebuilder:validation:Required
	Id string `json:"id"`

	// +kubebuilder:validation:Required
	Ip string `json:"ip"`

	// +kubebuilder:validation:Required
	Mac string `json:"mac"`

	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// +kubebuilder:validation:Optional
	Relateips []IpItem `json:"relate-ips,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="spidernodes",scope="Cluster",shortName={sn},singular="spidernode"
// +kubebuilder:printcolumn:JSONPath=".spec.pre-allocated",description="preallocated",name="PREALLOCATED",type=string
// +kubebuilder:printcolumn:JSONPath=".status.allocatedIPCount",description="allocatedIPCount",name="ALLOCATED-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.totalIPCount",description="totalIPCount",name="TOTAL-IP-COUNT",type=integer
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:nonNamespaced

// SpiderNode is the Schema for the spidernodes API.
type SpiderNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSpec   `json:"spec,omitempty"`
	Status NodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderNodeList contains a list of SpiderNode.
type SpiderNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderNode{}, &SpiderNodeList{})
}
