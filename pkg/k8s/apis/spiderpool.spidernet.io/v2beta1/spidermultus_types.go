// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:resource:categories={spiderpool},path="spidermultusconfigs",scope="Namespaced",shortName={smc},singular="spidermultusconfig"
// +kubebuilder:object:root=true
// +genclient
// +genclient:noStatus
type SpiderMultusConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the MultusCNIConfig
	Spec MultusCNIConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type SpiderMultusConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SpiderMultusConfig `json:"items"`
}

// calico、cilium-cni、flannel、weave-net、kube-ovn、macvlan、ipvlan、sriov、custom
type CniType string

// MultusCNIConfigSpec defines the desired state of SpiderMultusConfig.
type MultusCNIConfigSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=calico;cilium-cni;flannel;weave-net;kube-ovn;macvlan;ipvlan;sriov;custom
	CniType CniType `json:"cniType"`

	// +kubebuilder:validation:Optional
	MacvlanConfig *SpiderMacvlanCniConfig `json:"macvlan,omitempty"`

	// +kubebuilder:validation:Optional
	IPVlanConfig *SpiderIPvlanCniConfig `json:"ipvlan,omitempty"`

	// +kubebuilder:validation:Optional
	SriovConfig *SpiderSRIOVCniConfig `json:"sriov,omitempty"`

	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	EnableCoordinator *bool `json:"enableCoordinator"`

	// +kubebuilder:validation:Optional
	CoordinatorConfig *CoordinatorSpec `json:"coordinator,omitempty"`

	// OtherCniTypeConfig only used for CniType custom, valid json format, can be empty
	// +kubebuilder:validation:Optional
	CustomCNIConfig *string `json:"customCNI,omitempty"`
}

type SpiderMacvlanCniConfig struct {
	// +kubebuilder:validation:Required
	Master []string `json:"master"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:validation:Optional
	Bond *BondConfig `json:"bond,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"spiderpoolConfigPools,omitempty"`
}

type SpiderIPvlanCniConfig struct {
	// +kubebuilder:validation:Required
	Master []string `json:"master"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:validation:Optional
	Bond *BondConfig `json:"bond,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"spiderpoolConfigPools,omitempty"`
}

type SpiderSRIOVCniConfig struct {
	// +kubebuilder:validation:Required
	ResourceName string `json:"resourceName"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"spiderpoolConfigPools,omitempty"`
}

type BondConfig struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=6
	Mode int32 `json:"mode"`

	// +kubebuilder:validation:Optional
	Options *string `json:"options,omitempty"`
}

// SpiderpoolPools could specify the IPAM spiderpool CNI configuration default IPv4&IPv6 pools.
type SpiderpoolPools struct {
	IPv4IPPool []string `json:"IPv4IPPool,omitempty"`
	IPv6IPPool []string `json:"IPv6IPPool,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SpiderMultusConfig{}, &SpiderMultusConfigList{})
}
