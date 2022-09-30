# SpiderIPPool

A SpiderIPPool resource represents a collection of IP addresses from which Spiderpool expects endpoint IPs to be assigned.

## CRD Definition

The SpiderIPPool custom resource is modeled after a standard Kubernetes resource
and is split into a ``spec`` and ``status`` section:

```text
type SpiderIPPool struct {
    [...]
    
    // Spec is the specification of the IPPool
    Spec   IPPoolSpec   `json:"spec,omitempty"`
    
    // Status is the status of the IPPool
    Status IPPoolStatus `json:"status,omitempty"`
}
```

### IPPool Specification

The ``spec`` section embeds an IPPool specific field which allows to define the list of all IPs, ExcludeIPs, Routes
and some other data to the IPPool object for allocation:

```text
// IPPoolSpec defines the desired state of SpiderIPPool
type IPPoolSpec struct {
    // specify the IPPool's IP version
    IPVersion *int64 `json:"ipVersion,omitempty"`

    // specify the IPPool's subnet
    Subnet string `json:"subnet"`

    // specify the IPPool's IP ranges
    IPs []string `json:"ips"`

    // determine whether ths IPPool could be used or not
    Disable *bool `json:"disable,omitempty"`

    // specify the exclude IPs for the IPPool
    ExcludeIPs []string `json:"excludeIPs,omitempty"`

    // specify the gateway
    Gateway *string `json:"gateway,omitempty"`

    // specify the vlan
    Vlan *int64 `json:"vlan,omitempty"`

    //specify the routes
    Routes []Route `json:"routes,omitempty"`

    PodAffinity *metav1.LabelSelector `json:"podAffinity,omitempty"`

    NamesapceAffinity *metav1.LabelSelector `json:"namespaceAffinity,omitempty"`

    NodeAffinity *metav1.LabelSelector `json:"nodeAffinity,omitempty"`
}

type Route struct {
    // destination
    Dst string `json:"dst"`
    
    // gateway
    Gw string `json:"gw"`
}
```

### IPPool Status

The ``status`` section contains some field to describe the current IPPool allocation details.
The IPPool status reports all used addresses.

```text
// IPPoolStatus defines the observed state of SpiderIPPool
type IPPoolStatus struct {
    // all used addresses details
    AllocatedIPs PoolIPAllocations `json:"allocatedIPs,omitempty"`

    // the IPPool total addresses counts
    TotalIPCount *int64 `json:"totalIPCount,omitempty"`

    // the IPPool used addresses counts
    AllocatedIPCount *int64 `json:"allocatedIPCount,omitempty"`
}

// PoolIPAllocations is a map of allocated IPs indexed by IP
type PoolIPAllocations map[string]PoolIPAllocation

// PoolIPAllocation is an IP already has been allocated
type PoolIPAllocation struct {
    // container ID
    ContainerID string `json:"containerID"`
    
    // interface name
    NIC string `json:"interface"`

    // node name
    Node string `json:"node"`

    // namespace
    Namespace string `json:"namespace"`

    // pod name
    Pod string `json:"pod"`

    // kubernetes controller owner reference
    OwnerControllerType string `json:"ownerControllerType"`
}
```
