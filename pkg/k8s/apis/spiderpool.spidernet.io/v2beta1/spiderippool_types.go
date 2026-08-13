// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPPoolSpec defines the desired state of SpiderIPPool.
type IPPoolSpec struct {
	// +kubebuilder:validation:Enum=4;6
	// +kubebuilder:validation:Optional
	IPVersion *int64 `json:"ipVersion,omitempty"`

	// +kubebuilder:validation:Required
	Subnet string `json:"subnet"`

	// +kubebuilder:validation:Optional
	IPs []string `json:"ips,omitempty"`

	// +kubebuilder:validation:Optional
	ExcludeIPs []string `json:"excludeIPs,omitempty"`

	// +kubebuilder:validation:Optional
	Gateway *string `json:"gateway,omitempty"`

	// +kubebuilder:validation:Optional
	Routes []Route `json:"routes,omitempty"`

	// +kubebuilder:validation:Optional
	PodAffinity *metav1.LabelSelector `json:"podAffinity,omitempty"`

	// +kubebuilder:validation:Optional
	NamespaceAffinity *metav1.LabelSelector `json:"namespaceAffinity,omitempty"`

	// +kubebuilder:validation:Optional
	NamespaceName []string `json:"namespaceName,omitempty"`

	// +kubebuilder:validation:Optional
	NodeAffinity *metav1.LabelSelector `json:"nodeAffinity,omitempty"`

	// +kubebuilder:validation:Optional
	NodeName []string `json:"nodeName,omitempty"`

	// +kubebuilder:validation:Optional
	MultusName []string `json:"multusName,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Default *bool `json:"default,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Disable *bool `json:"disable,omitempty"`
}

type Route struct {
	// +kubebuilder:validation:Required
	Dst string `json:"dst"`

	// +kubebuilder:validation:Required
	Gw string `json:"gw"`
}

// IPPoolStatus defines the observed state of SpiderIPPool.
type IPPoolStatus struct {
	// +kubebuilder:validation:Optional
	AllocatedIPs *string `json:"allocatedIPs,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	TotalIPCount *int64 `json:"totalIPCount,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	AllocatedIPCount *int64 `json:"allocatedIPCount,omitempty"`

	// IPMetaData carries per-IP link-layer/pairing metadata written by an
	// external IaaS provider controller for prewarm-managed pools. It is
	// provider-owned: Spiderpool consumes it read-only and never writes to
	// it. For a paired (dual-stack) pool set, this field exists only on the
	// primary pool (by convention the v4 pool); the sibling pool's own
	// ipMetaData is left empty by the provider. Presence of an address as a
	// key in Metadata IS the ready state; prewarm failure is expressed
	// purely as absence from Metadata (counted in UnreadyIPCount), with
	// per-IP failure detail in provider logs only.
	// +kubebuilder:validation:Optional
	IPMetaData *IPMetaData `json:"ipMetaData,omitempty"`
}

// IPMetaData is the provider-owned per-IP metadata block of an IaaS-managed
// SpiderIPPool (see IPPoolStatus.IPMetaData).
type IPMetaData struct {
	// Metadata is a JSON-encoded map from a prewarmed address to its
	// link-layer/pairing metadata. The key is the primary-family address:
	// IPv4 for v4/primary pools, and IPv6 only for a pure-v6 single-stack
	// pool. Presence of a key in the decoded map IS the ready state.
	// Besides address keys, the map reserves the standalone key
	// "parentNic", whose string value is the pool-level parent NIC name on
	// the node this pool is bound to, from which the provider derives
	// sub-interfaces/sub-IPs.
	// +kubebuilder:validation:Optional
	Metadata *string `json:"metadata,omitempty"`

	// ObservedGeneration is the pool generation for which the provider
	// completed a trustworthy full evaluation. Individual IP failures are
	// represented by absent metadata entries and UnreadyIPCount.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// ReadyIPCount is the number of IPs that have a Metadata entry
	// (= successfully prewarmed). Provider-written, observational only —
	// it never gates allocation.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	ReadyIPCount *int64 `json:"readyIPCount,omitempty"`

	// UnreadyIPCount is the number of IPs in spec.ips that have NO Metadata
	// entry (= unready or prewarm-failed). Provider-written, observational
	// only — it never gates allocation.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	UnreadyIPCount *int64 `json:"unreadyIPCount,omitempty"`
}

// IPMetadataEntry is the metadata attached to one (possibly paired) IP.
type IPMetadataEntry struct {
	// IPv6 is the paired IPv6 address for dual-stack paired pools; absent
	// for single-stack entries.
	// +kubebuilder:validation:Optional
	IPv6 *string `json:"ipv6,omitempty"`

	// +kubebuilder:validation:Optional
	MAC string `json:"mac,omitempty"`

	// +kubebuilder:validation:Optional
	VLAN *int32 `json:"vlan,omitempty"`
}

// PoolIPAllocations is a map of IP allocation details indexed by IP address.
type PoolIPAllocations map[string]PoolIPAllocation

type PoolIPAllocation struct {
	NamespacedName string `json:"pod"`
	PodUID         string `json:"podUid"`
}

// +kubebuilder:resource:categories={spiderpool},path="spiderippools",scope="Cluster",shortName={sp},singular="spiderippool"
// +kubebuilder:printcolumn:JSONPath=".spec.ipVersion",description="ipVersion",name="VERSION",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.subnet",description="subnet",name="SUBNET",type=string
// +kubebuilder:printcolumn:JSONPath=".status.allocatedIPCount",description="allocatedIPCount",name="ALLOCATED-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".status.totalIPCount",description="totalIPCount",name="TOTAL-IP-COUNT",type=integer
// +kubebuilder:printcolumn:JSONPath=".spec.default",description="default",name="DEFAULT",type=boolean
// +kubebuilder:printcolumn:JSONPath=".spec.disable",description="disable",name="DISABLE",type=boolean,priority=10
// +kubebuilder:printcolumn:JSONPath=".spec.nodeName",description="nodeName",name="NodeName",type=string,priority=10
// +kubebuilder:printcolumn:JSONPath=".spec.multusName",description="multusName",name="MultusName",type=string,priority=10
// +kubebuilder:printcolumn:JSONPath=`.spec.podAffinity.matchLabels['ipam\.spidernet\.io/app\-namespace']`,description="AppNamespace",name="APP-NAMESPACE",type=string,priority=10
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// SpiderIPPool is the Schema for the spiderippools API.
type SpiderIPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpiderIPPoolList contains a list of SpiderIPPool.
type SpiderIPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderIPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiderIPPool{}, &SpiderIPPoolList{})
}
