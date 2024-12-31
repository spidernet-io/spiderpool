// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPPoolSpec defines the desired state of SpiderIPPool.
type IPPoolSpec struct {
	// +kubebuilder:validation:Enum=4;6
	// +kubebuilder:validation:Optional
	IPVersion *int64 `json:"ipVersion,omitempty"`

	// +kubebuilder:validation:Required
	Subnet string `json:"subnet"`

	// +kubebuilder:validation:Optional
	IPs []string `json:"ips,omitempty"`

	// +kubebuilder:validation:Optional
	ExcludeIPs []string `json:"excludeIPs,omitempty"`

	// +kubebuilder:validation:Optional
	Gateway *string `json:"gateway,omitempty"`

	// DEPRECATED: Vlan is deprecated.
	// +kubebuilder:validation:Maximum=4094
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	Vlan *int64 `json:"vlan,omitempty"`

	// +kubebuilder:validation:Optional
	Routes []Route `json:"routes,omitempty"`

	// +kubebuilder:validation:Optional
	PodAffinity *metav1.LabelSelector `json:"podAffinity,omitempty"`

	// +kubebuilder:validation:Optional
	NamespaceAffinity *metav1.LabelSelector `json:"namespaceAffinity,omitempty"`

	// +kubebuilder:validation:Optional
	NamespaceName []string `json:"namespaceName,omitempty"`

	// +kubebuilder:validation:Optional
	NodeAffinity *metav1.LabelSelector `json:"nodeAffinity,omitempty"`

	// +kubebuilder:validation:Optional
	NodeName []string `json:"nodeName,omitempty"`

	// +kubebuilder:validation:Optional
	MultusName []string `json:"multusName,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Default *bool `json:"default,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Disable *bool `json:"disable,omitempty"`
}

type Route struct {
	// +kubebuilder:validation:Required
	Dst string `json:"dst"`

	// +kubebuilder:validation:Required
	Gw string `json:"gw"`
}

// IPPoolStatus defines the observed state of SpiderIPPool.
type IPPoolStatus struct {
	// +kubebuilder:validation:Optional
	AllocatedIPs *string `json:"allocatedIPs,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int64 `json:"totalIPCount,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocatedIPCount *int64 `json:"allocatedIPCount,omitempty"`
}

// PoolIPAllocations is a map of IP allocation details indexed by IP address.
type PoolIPAllocations map[string]PoolIPAllocation

type PoolIPAllocation struct {
	NamespacedName string `json:"pod"`
	PodUID         string `json:"podUid"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderippools",scope="Cluster",shortName={sp},singular="spiderippool"
// +kubebuilder:printcolumn:JSONPath=".spec.ipVersion",description="ipVersion",name="VERSION",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.subnet",description="subnet",name="SUBNET",type=string
// +kubebuilder:printcolumn:JSONPath=".status.allocatedIPCount",description="allocatedIPCount",name="ALLOCATED-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.totalIPCount",description="totalIPCount",name="TOTAL-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".spec.default",description="default",name="DEFAULT",type=boolean
// +kubebuilder:printcolumn:JSONPath=".spec.disable",description="disable",name="DISABLE",type=boolean,priority=10
// +kubebuilder:printcolumn:JSONPath=".spec.nodeName",description="nodeName",name="NodeName",type=string,priority=10
// +kubebuilder:printcolumn:JSONPath=".spec.multusName",description="multusName",name="MultusName",type=string,priority=10
// +kubebuilder:printcolumn:JSONPath=`.spec.podAffinity.matchLabels['ipam\.spidernet\.io/app\-namespace']`,description="AppNamespace",name="APP-NAMESPACE",type=string,priority=10
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:nonNamespaced

// SpiderIPPool is the Schema for the spiderippools API.
type SpiderIPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderIPPoolList contains a list of SpiderIPPool.
type SpiderIPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderIPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderIPPool{}, &SpiderIPPoolList{})
}
