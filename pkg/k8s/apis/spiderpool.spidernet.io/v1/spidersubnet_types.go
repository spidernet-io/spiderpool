// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SubnetSpec defines the desired state of SpiderSubnet
type SubnetSpec struct {
	// +kubebuilder:validation:Enum=4;6
	// +kubebuilder:validation:Optional
	IPVersion *int64 `json:"ipVersion,omitempty"`

	// +kubebuilder:validation:Required
	Subnet string `json:"subnet"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	IPs []string `json:"ips"`

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
}

// SubnetStatus defines the observed state of SpiderSubnet
type SubnetStatus struct {
	// +kubebuilder:validation:Optional
	FreeIPs []string `json:"freeIPs,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int64 `json:"totalIPCount,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	FreeIPCount *int64 `json:"freeIPCount,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="spidersubnets",scope="Cluster",shortName={ss},singular="spidersubnet"
// +kubebuilder:printcolumn:JSONPath=".spec.ipVersion",description="ipVersion",name="VERSION",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.subnet",description="subnet",name="SUBNET",type=string
// +kubebuilder:printcolumn:JSONPath=".status.freeIPCount",description="freeIPCount",name="FREE-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.totalIPCount",description="totalIPCount",name="TOTAL-IP-COUNT",type=integer
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:nonNamespaced

// SpiderSubnet is the Schema for the spidersubnets API
type SpiderSubnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec,omitempty"`
	Status SubnetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderSubnetList contains a list of SpiderSubnet
type SpiderSubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderSubnet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderSubnet{}, &SpiderSubnetList{})
}
