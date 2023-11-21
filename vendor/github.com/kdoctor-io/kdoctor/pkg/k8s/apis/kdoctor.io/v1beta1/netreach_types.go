// Copyright 2023 Authors of kdoctor-io
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetReachSpec struct {
	// for the nested field, you should add the kubebuilder default tag even if the nested field properties own the default value.

	// +kubebuilder:validation:Optional
	AgentSpec *AgentSpec `json:"agentSpec,omitempty"`

	// +kubebuilder:validation:Optional
	Schedule *SchedulePlan `json:"schedule,omitempty"`

	// +kubebuilder:validation:Optional
	Target *NetReachTarget `json:"target,omitempty"`

	// +kubebuilder:validation:Optional
	Request *NetHttpRequest `json:"request,omitempty"`

	// +kubebuilder:validation:Optional
	SuccessCondition *NetSuccessCondition `json:"expect,omitempty"`
}

type NetReachTarget struct {
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	IPv4 *bool `json:"ipv4,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	IPv6 *bool `json:"ipv6,omitempty"`

	// +kubebuilder:default=true
	Endpoint *bool `json:"endpoint,omitempty"`

	// +kubebuilder:default=false
	MultusInterface *bool `json:"multusInterface,omitempty"`

	// +kubebuilder:default=true
	ClusterIP *bool `json:"clusterIP,omitempty"`

	// +kubebuilder:default=true
	NodePort *bool `json:"nodePort,omitempty"`

	// +kubebuilder:default=false
	LoadBalancer *bool `json:"loadBalancer,omitempty"`

	// +kubebuilder:default=false
	Ingress *bool `json:"ingress,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	EnableLatencyMetric bool `json:"enableLatencyMetric,omitempty"`
}

// scope(Namespaced or Cluster)
// +kubebuilder:resource:categories={kdoctor},path="netreaches",singular="netreach",shortName={nr},scope="Cluster"
// +kubebuilder:printcolumn:JSONPath=".status.finish",description="finish",name="finish",type=boolean
// +kubebuilder:printcolumn:JSONPath=".status.expectedRound",description="expectedRound",name="expectedRound",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.doneRound",description="doneRound",name="doneRound",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.lastRoundStatus",description="lastRoundStatus",name="lastRoundStatus",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.schedule.schedule",description="schedule",name="schedule",type=string
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:nonNamespaced

type NetReach struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   NetReachSpec `json:"spec,omitempty"`
	Status TaskStatus   `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NetReachList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NetReach `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetReach{}, &NetReachList{})
}
