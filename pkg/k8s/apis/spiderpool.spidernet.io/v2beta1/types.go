// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClaimParameterSpec defines the desired state of SpiderClaimParameter.
type ClaimParameterSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	RdmaTCPAccelerate bool `json:"rdmaTCPAccelerate,omitempty"`

	// +kubebuilder:validation:Optional
	MultusNames []string `json:"multusNames,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderIPPools []string `json:"ippools,omitempty"`

	// +kubebuilder:validation:Optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderclaimparameters",scope="Namespaced",shortName={scp},singular="spiderclaimparameter"
// +kubebuilder:object:root=true
// +genclient:noStatus

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
