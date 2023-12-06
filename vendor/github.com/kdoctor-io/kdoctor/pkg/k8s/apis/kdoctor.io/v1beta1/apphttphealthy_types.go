// Copyright 2023 Authors of kdoctor-io
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppHttpHealthySpec struct {
	// for the nested field, you should add the kubebuilder default tag even if the nested field properties own the default value.

	// +kubebuilder:validation:Optional
	AgentSpec *AgentSpec `json:"agentSpec,omitempty"`

	// +kubebuilder:validation:Optional
	Schedule *SchedulePlan `json:"schedule,omitempty"`

	// +kubebuilder:validation:Optional
	Target *AppHttpHealthyTarget `json:"target,omitempty"`

	// +kubebuilder:validation:Optional
	Request *NetHttpRequest `json:"request,omitempty"`

	// +kubebuilder:validation:Optional
	SuccessCondition *NetSuccessCondition `json:"expect,omitempty"`
}

type AppHttpHealthyTarget struct {

	// +kubebuilder:validation:Type:=string
	Host string `json:"host"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Enum=GET;POST;PUT;DELETE;CONNECT;OPTIONS;PATCH;HEAD
	Method string `json:"method"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Http2 bool `json:"http2"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Optional
	BodyConfigName *string `json:"bodyConfigmapName,omitempty"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Optional
	BodyConfigNamespace *string `json:"bodyConfigmapNamespace,omitempty"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Optional
	TlsSecretName *string `json:"tlsSecretName,omitempty"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Optional
	TlsSecretNamespace *string `json:"tlsSecretNamespace,omitempty"`

	// +kubebuilder:validation:Optional
	Header []string `json:"header,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	EnableLatencyMetric bool `json:"enableLatencyMetric,omitempty"`
}

// scope(Namespaced or Cluster)
// +kubebuilder:resource:categories={kdoctor},path="apphttphealthies",singular="apphttphealthy",shortName={ahh},scope="Cluster"
// +kubebuilder:printcolumn:JSONPath=".status.finish",description="finish",name="finish",type=boolean
// +kubebuilder:printcolumn:JSONPath=".status.expectedRound",description="expectedRound",name="expectedRound",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.doneRound",description="doneRound",name="doneRound",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.lastRoundStatus",description="lastRoundStatus",name="lastRoundStatus",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.schedule.schedule",description="schedule",name="schedule",type=string
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:nonNamespaced

type AppHttpHealthy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   AppHttpHealthySpec `json:"spec,omitempty"`
	Status TaskStatus         `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type AppHttpHealthyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AppHttpHealthy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppHttpHealthy{}, &AppHttpHealthyList{})
}
