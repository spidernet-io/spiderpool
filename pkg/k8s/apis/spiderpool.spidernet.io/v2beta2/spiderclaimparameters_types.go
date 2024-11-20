// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClaimParameterSpec defines the desired state of SpiderClaimParameter.
type ClaimParameterSpec struct {
	// EnableRDMA If it is true, then all SpiderMultusConfig references in
	// this SpiderClaimParameter must be enabled.
	// +kubebuilder:default=false
	EnableRDMA bool `json:"enableRdma"`

	// DefaultNic aSpecify which SpiderMultusConfig is to be used as the
	// default NIC for the pod.
	// +kubebuilder:validation:Optional
	DefaultNic *MultusConfig `json:"defaultNic"`

	// SecondaryNics a list of SpiderMultusConfig references that are to be
	// used as secondary NICs for the pod.
	// +kubebuilder:validation:Optional
	SecondaryNics []MultusConfig `json:"secondaryNics"`
}

type MultusConfig struct {
	// MultusName the name of the SpiderMultusConfig instance
	// +kubebuilder:validation:Required
	MultusName string `json:"multusName"`
	// Namespace the namespace of the SpiderMultusConfig instance
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// DefaultRoute indicated whether this nic is the default route nic for the pod
	// +kubebuilder:validation:Optional
	DefaultRoute bool `json:"defaultRoute"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderclaimparameters",scope="Namespaced",shortName={scp},singular="spiderclaimparameter"
// +kubebuilder:object:root=true
// +genclient:noStatus
// +genclient

// SpiderClaimParameter is the Schema for the spiderclaimparameters API.
type SpiderClaimParameter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClaimParameterSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderClaimParameterList contains a list of SpiderClaimParameter.
type SpiderClaimParameterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderClaimParameter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderClaimParameter{}, &SpiderClaimParameterList{})
}
