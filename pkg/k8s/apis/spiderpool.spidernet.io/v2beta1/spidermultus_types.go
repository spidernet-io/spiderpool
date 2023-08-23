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

// MultusCNIConfigSpec defines the desired state of SpiderMultusConfig.
type MultusCNIConfigSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=macvlan;ipvlan;sriov;ovs;custom
	CniType string `json:"cniType"`

	// +kubebuilder:validation:Optional
	MacvlanConfig *SpiderMacvlanCniConfig `json:"macvlan,omitempty"`

	// +kubebuilder:validation:Optional
	IPVlanConfig *SpiderIPvlanCniConfig `json:"ipvlan,omitempty"`

	// +kubebuilder:validation:Optional
	SriovConfig *SpiderSRIOVCniConfig `json:"sriov,omitempty"`

	// +kubebuilder:validation:Optional
	OvsConfig *SpiderOvsCniConfig `json:"ovs,omitempty"`

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
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
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
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderSRIOVCniConfig struct {
	// +kubebuilder:validation:Required
	ResourceName string `json:"resourceName"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderOvsCniConfig struct {
	// +kubebuilder:validation:Required
	BrName string `json:"bridge"`
	// +kubebuilder:validation:Optional
	VlanTag *int32 `json:"vlan"`
	// +kubebuilder:validation:Optional
	Trunk []*Trunk `json:"trunk,omitempty"`
	// +kubebuilder:validation:Optional
	// PCI address of a VF in valid sysfs format
	DeviceID string `json:"deviceID"`
	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type Trunk struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	MinID *uint `json:"minID,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	MaxID *uint `json:"maxID,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	ID *uint `json:"id,omitempty"`
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
	// +kubebuilder:validation:Optional
	IPv4IPPool []string `json:"ipv4,omitempty"`

	// +kubebuilder:validation:Optional
	IPv6IPPool []string `json:"ipv6,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SpiderMultusConfig{}, &SpiderMultusConfigList{})
}
