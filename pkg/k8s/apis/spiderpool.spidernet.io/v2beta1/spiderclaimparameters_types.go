// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClaimParameterSpec defines the desired state of SpiderClaimParameter.
type ClaimParameterSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	RdmaAcc bool `json:"rdmaAcc,omitempty"`

	// +kubebuilder:validation:Optional
	StaticNics []StaticNic `json:"staticNics,omitempty"`
}

type StaticNic struct {
	// +kubebuilder:validation:Required
	MultusConfigName string `json:"multusConfigName"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
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
