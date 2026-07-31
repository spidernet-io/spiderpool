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

	// Deprecated: use spec.ipam.enabled instead. If spec.ipam.enabled is set,
	// it takes precedence over this field.
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	DisableIPAM *bool `json:"disableIPAM,omitempty"`

	// IPAM configures the spiderpool IPAM plugin in the generated
	// network-attachment-definition.
	// +kubebuilder:validation:Optional
	IPAM *SpiderIPAMConfig `json:"ipam,omitempty"`

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

// SpiderIPAMConfig configures the spiderpool IPAM plugin section of the
// generated network-attachment-definition.
type SpiderIPAMConfig struct {
	// Enabled indicates whether the spiderpool IPAM plugin is enabled in the
	// generated network-attachment-definition. Default true. If set, it takes
	// precedence over the deprecated spec.disableIPAM field.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// LogOptions configures the logging of the spiderpool IPAM plugin.
	// +kubebuilder:validation:Optional
	LogOptions *LogOptions `json:"logOptions,omitempty"`
}

// LogOptions configures the file logging of a CNI plugin.
type LogOptions struct {
	// LogLevel configures the log level. If unset, the plugin uses its own
	// default log level.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=debug;info;warn;error
	LogLevel *string `json:"logLevel,omitempty"`

	// LogFilePath configures the path of the log file. If unset, the plugin
	// uses its own default log file path.
	// +kubebuilder:validation:Optional
	LogFilePath *string `json:"logFilePath,omitempty"`

	// LogFileMaxSize configures the maximum size in megabytes of a log file
	// before it gets rotated. nil means the default (100). 0 means the
	// rotation library default.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	LogFileMaxSize *int32 `json:"logFileMaxSize,omitempty"`

	// LogFileMaxAge configures the maximum number of days to retain old
	// rotated log files. nil means the default (30). 0 means retaining
	// old log files with no day limit.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	LogFileMaxAge *int32 `json:"logFileMaxAge,omitempty"`

	// LogFileMaxCount configures the maximum number of old rotated log files
	// to retain. nil means the default (10). 0 means retaining all old log
	// files.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	LogFileMaxCount *int32 `json:"logFileMaxCount,omitempty"`
}

type SpiderMacvlanCniConfig struct {
	// +kubebuilder:validation:Required
	// The master interface(s) for the CNI configuration. At least one master interface must be specified.
	// If multiple master interfaces are specified, the spiderpool will create a bond device with the bondConfig
	// by the ifacer plugin.
	Master []string `json:"master"`

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
	// enable IPAM to check if the IPPools of the pod if matched the master subnet
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// +kubebuilder:validation:Enum=true;false
	MatchMasterSubnet *bool `json:"matchMasterSubnet,omitempty"`

	// +kubebuilder:validation:Optional
	IPv4IPPool []string `json:"ipv4,omitempty"`

	// +kubebuilder:validation:Optional
	IPv6IPPool []string `json:"ipv6,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SpiderMultusConfig{}, &SpiderMultusConfigList{})
}
