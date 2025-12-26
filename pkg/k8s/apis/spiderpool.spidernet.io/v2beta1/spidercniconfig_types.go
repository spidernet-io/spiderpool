// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:resource:categories={spiderpool},path="spidercniconfigs",scope="Cluster",shortName={scc},singular="spidercniconfig"
// +kubebuilder:object:root=true

// +genclient
// +genclient:noStatus
type SpiderCNIConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the CNI configuration
	Spec MultusCNIConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type SpiderCNIConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderCNIConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderCNIConfig{}, &SpiderCNIConfigList{})
}
