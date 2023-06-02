# Multus CNI Config

A MultusCNIConfig resource represents the configuration information of a multus Network-Attachment-Definition.

## Why

There are several reasons to use multusCniConfig CRD for Multus Network-Attachment-Definition:

- Simplify the creation of cni config in multus CR, and also provide webhook verification of the cni configuration, multus doesn't provide webhook for multus CR.
- It's good for ipvlan、macvlan、sriov,etc. work with coordinator. and simplify the creation of coordinator config.
- More flexibility and scalability.

## CRD definition

The MultusCNIConfig custom resource is modeled after a standard Kubernetes resource
and is split into a `spec` section:

```go
type MultusCNIConfig struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    // Spec is the specification of the MultusCNIConfig
    Spec   MultusConfigSpec   `json:"spec,omitempty"`
}
```

### MultusCNIConfigSpec 

```go
// [macvlan、ipvlan、sriov、calico、cilium、custom]
type CniType string 

type MultusCNIConfigSpec struct {
    CniType CniType
    MacvlanConfig *VlanCniConfig
    IPVlanConfig *VlanCniConfig
    SriovConfig *SriovConfig
    // only for CniType: custom
    // valid json format, can be empty
    OtherCniTypeConfig string
    // for Coordinator
    CoordinatorConfig *CoordinatorConfig
}

// IPVlan and Macvlan
type VlanCniConfig struct {
    Master string 
    VlanId int
}

type SriovConfig struct {
    ResourceName string
    VlanId int
}

// source: api/models/coordinator_config.go: L23
type CoordinatorConfig struct {
    // detect gateway 
    DetectGateway bool `json:"detectGateway,omitempty"`
    // detect IP conflict
    DetectIPConflict bool `json:"detectIPConflict,omitempty"`
    // extra c ID r
    ExtraCIDR []string `json:"extraCIDR"`
    // host r p filter
    HostRPFilter int64 `json:"hostRPFilter,omitempty"`
    // host rule table
    HostRuleTable int64 `json:"hostRuleTable,omitempty"`
    // pod c ID r
    // Required: true
    PodCIDR []string `json:"podCIDR"`
    // pod default route n i c
    PodDefaultRouteNIC string `json:"podDefaultRouteNIC,omitempty"`
    // pod m a c prefix
    PodMACPrefix string `json:"podMACPrefix,omitempty"`
    // service c ID r
    // Required: true
    ServiceCIDR []string `json:"serviceCIDR"`
    // tune mode
    // Required: true
    TuneMode *string `json:"tuneMode"`
    // tune pod routes
    // Required: true
    TunePodRoutes *bool `json:"tunePodRoutes"`
}
```

### Reconcile 

_MultusCNIConfig_ and Multus _Network-Attachment-Definition_ are one-to-one mappings, So we need to sync their state. 

- All annotations and labels of MultusCNIConfig should be synced into the multus CR.
- ipam.spidernet.io/multus-cr-name is a special label that stores the mapping with the name of the multus cr. For example, there are a MultusCNIConfig cr with name: `multuscNIConfig1`, and with label: `ipam.spidernet.io/multus-cr-name: multusCR1`. So a multus CR named `multusCR1` needs to be created. 
- The life cycle of a multusCR created by `MultusCNIConfig` should be managed by `MultusCNIConfig`. Notice: But in the case of multusCR created by MultusCNIConfig but manually deleted, MultusCNIConfig should also be automatically gc.
