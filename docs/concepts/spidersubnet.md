# SpiderSubnet

A SpiderSubnet resource represents a collection of IP addresses from which Spiderpool expects SpiderIPPool IPs to be assigned.

## CRD definition

The SpiderSubnet custom resource is modeled after a standard Kubernetes resource
and is split into a `spec` and a `status` section:

```text
type SpiderSubnet struct {
    [...]

    // Spec is the specification of the Subnet
    Spec   SubnetSpec   `json:"spec,omitempty"`

    // Status is the status of the SpiderSubnet
    Status SubnetStatus `json:"status,omitempty"`
}
```

### Subnet spec

The `spec` section embeds a specific Subnet field which allows to define the list of all IPs, ExcludeIPs, Routes,
and some other data to the Subnet object for allocation:

```text
// SubnetSpec defines the desired state of SpiderSubnet
type SubnetSpec struct {
    // specify the SpiderSubnet's IP version
    IPVersion *int64 `json:"ipVersion,omitempty"`

    // specify the SpiderSubnet's subnet
    Subnet string `json:"subnet"`

    // specify the SpiderSubnet's IP ranges
    IPs []string `json:"ips"`

    // specify the exclude IPs for the SpiderSubnet
    ExcludeIPs []string `json:"excludeIPs,omitempty"`

    // specify the gateway
    Gateway *string `json:"gateway,omitempty"`

    // specify the vlan
    Vlan *int64 `json:"vlan,omitempty"`

    //specify the routes
    Routes []Route `json:"routes,omitempty"`
}
```

### Subnet status

The `status` section contains some fields to describe details about the current IPPool allocation.
The IPPool status reports all used addresses.

```text
// SubnetStatus defines the observed state of SpiderSubnet
type SubnetStatus struct {
    // the SpiderSubnet IPPool pre-allocations
    ControlledIPPools PoolIPPreAllocations `json:"controlledIPPools,omitempty"`

    // the SpiderSubnet total addresses counts
    TotalIPCount *int64 `json:"totalIPCount,omitempty"`

    // the SpiderSubnet allocated addresses counts
    AllocatedIPCount *int64 `json:"allocatedIPCount,omitempty"`
}
```

```text
// PoolIPPreAllocations is a map of pool IP pre-allocation details indexed by pool name.
type PoolIPPreAllocations map[string]PoolIPPreAllocation

type PoolIPPreAllocation struct {
    // specify the SpiderSubnet's IPPool allocation IP ranges
    IPs []string `json:"ips"`
}
```

