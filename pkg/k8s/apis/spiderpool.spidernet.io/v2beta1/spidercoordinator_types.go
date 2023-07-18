// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CoordinationSpec defines the desired state of SpiderCoordinator.
type CoordinatorSpec struct {
	// +kubebuilder:default=underlay
	// +kubebuilder:validation:Enum=underlay;overlay;disabled
	// +kubebuilder:validation:Optional
	Mode *string `json:"mode,omitempty"`

	// +kubebuilder:validation:Required
	PodCIDRType string `json:"podCIDRType"`

	// +kubebuilder:validation:Optional
	HijackCIDR []string `json:"hijackCIDR,omitempty"`

	// +kubebuilder:validation:Optional
	PodMACPrefix *string `json:"podMACPrefix,omitempty"`

	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	TunePodRoutes *bool `json:"tunePodRoutes,omitempty"`

	// +kubebuilder:validation:Optional
	PodDefaultRouteNIC *string `json:"podDefaultRouteNIC,omitempty"`

	// +kubebuilder:default=500
	// +kubebuilder:validation:Optional
	HostRuleTable *int `json:"hostRuleTable,omitempty"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Optional
	HostRPFilter *int `json:"hostRPFilter,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	DetectIPConflict *bool `json:"detectIPConflict,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	DetectGateway *bool `json:"detectGateway,omitempty"`
}

// CoordinationStatus defines the observed state of SpiderCoordinator.
type CoordinatorStatus struct {
	// +kubebuilder:validation:Requred
	Phase string `json:"phase"`

	// +kubebuilder:validation:Optional
	OverlayPodCIDR []string `json:"overlayPodCIDR,omitempty"`

	// +kubebuilder:validation:Optional
	ServiceCIDR []string `json:"serviceCIDR,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="spidercoordinators",scope="Cluster",shortName={scc},singular="spidercoordinator"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:nonNamespaced

// SpiderCoordinator is the Schema for the spidercoordinators API.
type SpiderCoordinator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CoordinatorSpec   `json:"spec,omitempty"`
	Status CoordinatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderCoordinatorList contains a list of SpiderCoordinator.
type SpiderCoordinatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderCoordinator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderCoordinator{}, &SpiderCoordinatorList{})
}
