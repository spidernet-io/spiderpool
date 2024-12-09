// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReservedIPSpec defines the desired state of SpiderReservedIP.
type ReservedIPSpec struct {
	// +kubebuilder:validation:Enum=4;6
	// +kubebuilder:validation:Optional
	IPVersion *int64 `json:"ipVersion,omitempty"`

	// +kubebuilder:validation:Optional
	IPs []string `json:"ips,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderreservedips",scope="Cluster",shortName={sr},singular="spiderreservedip"
// +kubebuilder:printcolumn:JSONPath=".spec.ipVersion",description="ipVersion",name="VERSION",type=string
// +kubebuilder:object:root=true

// SpiderReservedIP is the Schema for the spiderreservedips API.
type SpiderReservedIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ReservedIPSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderReservedIPList contains a list of SpiderReservedIP.
type SpiderReservedIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderReservedIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderReservedIP{}, &SpiderReservedIPList{})
}
