// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CoordinationSpec defines the desired state of SpiderCoordinator.
type CoordinatorSpec struct {
	// +kubebuilder:validation:Enum=auto;underlay;overlay;disabled
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=auto
	Mode *string `json:"mode,omitempty"`

	// CoordinatorSpec is used by SpiderCoordinator and SpiderMultusConfig
	// in spidermultusconfig CRD , podCIDRType should not be required, which could be merged from SpiderCoordinator CR
	// but in SpiderCoordinator CRD, podCIDRType should be required
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=auto;cluster;calico;cilium;none
	PodCIDRType *string `json:"podCIDRType,omitempty"`

	// +kubebuilder:validation:Optional
	HijackCIDR []string `json:"hijackCIDR,omitempty"`

	// +kubebuilder:validation:Optional
	PodMACPrefix *string `json:"podMACPrefix,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	TunePodRoutes *bool `json:"tunePodRoutes,omitempty"`

	// +kubebuilder:validation:Optional
	PodDefaultRouteNIC *string `json:"podDefaultRouteNIC,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=500
	HostRuleTable *int `json:"hostRuleTable,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	HostRPFilter *int `json:"hostRPFilter,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	TxQueueLen *int `json:"txQueueLen,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	DetectIPConflict *bool `json:"detectIPConflict,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	DetectGateway *bool `json:"detectGateway,omitempty"`
}

// CoordinationStatus defines the observed state of SpiderCoordinator.
type CoordinatorStatus struct {
	// +kubebuilder:validation:Requred
	Phase string `json:"phase"`

	// +kubebuilder: validation:Optional
	Reason string `json:"reason,omitempty"`

	// +kubebuilder:validation:Optional
	OverlayPodCIDR []string `json:"overlayPodCIDR"`

	// +kubebuilder:validation:Optional
	ServiceCIDR []string `json:"serviceCIDR"`
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
