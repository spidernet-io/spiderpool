// Copyright 2025 Authors of spidernet-io
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
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=macvlan;ipvlan;sriov;ovs;ib-sriov;ipoib;custom
	// +kubebuilder:default=custom
	CniType *string `json:"cniType,omitempty"`

	// +kubebuilder:validation:Optional
	MacvlanConfig *SpiderMacvlanCniConfig `json:"macvlan,omitempty"`

	// +kubebuilder:validation:Optional
	IPVlanConfig *SpiderIPvlanCniConfig `json:"ipvlan,omitempty"`

	// +kubebuilder:validation:Optional
	SriovConfig *SpiderSRIOVCniConfig `json:"sriov,omitempty"`

	// +kubebuilder:validation:Optional
	OvsConfig *SpiderOvsCniConfig `json:"ovs,omitempty"`

	// +kubebuilder:validation:Optional
	IbSriovConfig *SpiderIBSriovCniConfig `json:"ibsriov,omitempty"`

	// +kubebuilder:validation:Optional
	IpoibConfig *SpiderIpoibCniConfig `json:"ipoib,omitempty"`

	// if CniType was set to custom, we'll mutate this field to be false
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	EnableCoordinator *bool `json:"enableCoordinator,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	DisableIPAM *bool `json:"disableIPAM,omitempty"`

	// +kubebuilder:validation:Optional
	CoordinatorConfig *CoordinatorSpec `json:"coordinator,omitempty"`

	// ChainCNIJsonData is used to configure the configuration of chain CNI.
	// format in json.
	// +kubebuilder:validation:Optional
	ChainCNIJsonData []string `json:"chainCNIJsonData"`

	// OtherCniTypeConfig only used for CniType custom, valid json format, can be empty
	// +kubebuilder:validation:Optional
	CustomCNIConfig *string `json:"customCNI,omitempty"`
}

type SpiderMacvlanCniConfig struct {
	// +kubebuilder:validation:Required
	// The master interface(s) for the CNI configuration. At least one master interface must be specified.
	// If multiple master interfaces are specified, the spiderpool will create a bond device with the bondConfig
	// by the ifacer plugin.
	Master []string `json:"master"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// explicitly set MTU to the specified value. Defaults('0' or no value provided) to the value chosen by the kernel.
	MTU *int32 `json:"mtu,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	// The VLAN ID for the CNI configuration, optional and must be within the specified range: [0.4096).
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:validation:Optional
	// Optional bond configuration for the CNI. It must not be nil if the multiple master interfaces are specified.
	Bond *BondConfig `json:"bond,omitempty"`

	// +kubebuilder:validation:Optional
	// The RDMA resource name of the nic. the RDMA resource is often reported to kubelet by the
	// k8s-rdma-shared-dev-plugin. when it is not empty and spiderpool podResourceInject feature
	// is enabled, spiderpool can automatically inject it into the container's resources via webhook.
	RdmaResourceName *string `json:"rdmaResourceName,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderIPvlanCniConfig struct {
	// +kubebuilder:validation:Required
	// The master interface(s) for the CNI configuration. At least one master interface must be specified.
	// If multiple master interfaces are specified, the spiderpool will create a bond device with the bondConfig
	// by the ifacer plugin.
	Master []string `json:"master"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// explicitly set MTU to the specified value. Defaults('0' or no value provided) to the value chosen by the kernel.
	MTU *int32 `json:"mtu,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	// The VLAN ID for the CNI configuration, optional and must be within the specified range: [0.4096).
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:validation:Optional
	// Optional bond configuration for the CNI. It must not be nil if the multiple master interfaces are specified.
	Bond *BondConfig `json:"bond,omitempty"`

	// +kubebuilder:validation:Optional
	// The RDMA resource name of the nic. the RDMA resource is often reported to kubelet by the
	// k8s-rdma-shared-dev-plugin. when it is not empty and spiderpool podResourceInject feature
	// is enabled, spiderpool can automatically inject it into the container's resources via webhook.
	RdmaResourceName *string `json:"rdmaResourceName,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderSRIOVCniConfig struct {
	// +kubebuilder:validation:Required
	// The SR-IOV RDMA resource name of the SpiderMultusConfig. the SR-IOV RDMA resource is often
	// reported to kubelet by the sriov-device-plugin.
	ResourceName *string `json:"resourceName,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	// The VLAN ID for the CNI configuration, optional and must be within the specified range: [0.4096).
	VlanID *int32 `json:"vlanID,omitempty"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// explicitly set MTU to the specified value via tuning plugin. Defaults('0' or no value provided) to the value chosen by the kernel.
	MTU *int32 `json:"mtu,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	MinTxRateMbps *int `json:"minTxRateMbps,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// Mbps, 0 = disable rate limiting
	MaxTxRateMbps *int `json:"maxTxRateMbps,omitempty"` // Mbps, 0 = disable rate limiting

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	// rdmaIsolation enable RDMA CNI plugin is intended to be run as a chained CNI plugin.
	// it ensures isolation of RDMA traffic from other workloads in the system by moving
	// the associated RDMA interfaces of the provided network interface to the container's
	// network namespace path.
	RdmaIsolation *bool `json:"rdmaIsolation,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderIBSriovCniConfig struct {
	// +kubebuilder:validation:Required
	// The SR-IOV RDMA resource name of the SpiderMultusConfig. the SR-IOV RDMA resource is often
	// reported to kubelet by the sriov-device-plugin.
	ResourceName *string `json:"resourceName,omitempty"`

	// +kubebuilder:validation:Optional
	// infiniBand pkey for VF, this field is used by ib-kubernetes to add pkey with
	// guid to InfiniBand subnet manager client e.g. Mellanox UFM, OpenSM
	Pkey *string `json:"pkey,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=enable
	// +kubebuilder:validation:Enum=auto;enable;disable
	// Enforces link state for the VF. Allowed values: auto, enable, disable.
	LinkState *string `json:"linkState,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	// rdmaIsolation enablw RDMA CNI plugin is intended to be run as a chained CNI plugin.
	// it ensures isolation of RDMA traffic from other workloads in the system by moving
	// the associated RDMA interfaces of the provided network interface to the container's
	// network namespace path.
	RdmaIsolation *bool `json:"rdmaIsolation,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Enforces ib-sriov-cni to work with ib-kubernetes.
	EnableIbKubernetes *bool `json:"enableIbKubernetes,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderIpoibCniConfig struct {
	// +kubebuilder:validation:Required
	// name of the host interface to create the link from.
	Master string `json:"master,omitempty"`

	// +kubebuilder:validation:Optional
	SpiderpoolConfigPools *SpiderpoolPools `json:"ippools,omitempty"`
}

type SpiderOvsCniConfig struct {
	// +kubebuilder:validation:Required
	BrName string `json:"bridge"`
	// +kubebuilder:validation:Optional
	VlanTag *int32 `json:"vlan,omitempty"`
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
