// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CoordinationSpec defines the desired state of SpiderCoordinator.
type CoordinatorSpec struct {
	// Mode mode specifies the mode in which the coordinator runs,
	// and the configurable values include auto (default), underlay,
	// overlay, disabled.
	// +kubebuilder:validation:Enum=auto;underlay;overlay;disabled
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=auto
	Mode *string `json:"mode,omitempty"`

	// CoordinatorSpec is used by SpiderCoordinator and SpiderMultusConfig
	// in spidermultusconfig CRD , podCIDRType should not be required, which
	// could be merged from SpiderCoordinator CR
	// but in SpiderCoordinator CRD, podCIDRType should be required
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=auto;cluster;calico;cilium;none
	PodCIDRType *string `json:"podCIDRType,omitempty"`

	// HijackCIDR configure static routing tables in the pod that target these
	// subnets to ensure that when the pod accesses these subnets, packets
	// are forwarded through the host network stack, such as nodelocaldns(169.254.0.0/16)
	// +kubebuilder:validation:Optional
	HijackCIDR []string `json:"hijackCIDR,omitempty"`

	// PodMACPrefix the fixed MAC address prefix, the length is two bytes.
	// the lowest bit of the first byte must be 0, which indicates the
	// unicast MAC address. example: 0a:1b
	// +kubebuilder:validation:Optional
	PodMACPrefix *string `json:"podMACPrefix,omitempty"`

	// TunePodRoutes specifies whether to tune pod routes of multiple NICs on pods.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	TunePodRoutes *bool `json:"tunePodRoutes,omitempty"`

	// PodDefaultRouteNIC PodDefaultRouteNIC is used to configure the NIC where
	// the pod's default route resides. the default value is empty, which means
	// the default route will remain at eth0.
	// +kubebuilder:validation:Optional
	PodDefaultRouteNIC *string `json:"podDefaultRouteNIC,omitempty"`

	// HostRuleTable specifies the table number of the routing table used
	// to configure the communication between the pod and the local node.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=500
	HostRuleTable *int `json:"hostRuleTable,omitempty"`

	// PodRPFilter is used for coordiantor to help set the rp_filter parameters of the pod.
	// Configurable values: <negative number>/0/1/2. negative number means leave it as it is.
	// the default value is 0.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	PodRPFilter *int `json:"podRPFilter,omitempty"`

	// TxQueueLen to set the tx_queue_len of the pod. requirement is a positive integer
	// the default value is 0, which means leaving it as it is.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	TxQueueLen *int `json:"txQueueLen,omitempty"`

	// VethLinkAddress configure a ipv4 link-local address
	// for veth0 device. empty means disable. default is empty.
	// Format is like 169.254.100.1
	VethLinkAddress *string `json:"vethLinkAddress,omitempty"`
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
// +kubebuilder:storageversion
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
