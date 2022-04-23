// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadEndpointStatus defines the observed state of WorkloadEndpoint
type WorkloadEndpointStatus struct {
}

// +kubebuilder:resource:categories={spiderpool},path="workloadendpoints",scope="Cluster",shortName={swe},singular="workloadendpoint"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WorkloadEndpoint is the Schema for the workloadendpoints API
type WorkloadEndpoint struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Status WorkloadEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadEndpointList contains a list of WorkloadEndpoint
type WorkloadEndpointList struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []WorkloadEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadEndpoint{}, &WorkloadEndpointList{})
}
