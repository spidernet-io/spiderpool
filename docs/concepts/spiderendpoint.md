# SpiderEndpoint

A SpiderEndpoint resource represents IP address allocation details for a specific endpoint object.

## CRD definition

The SpiderEndpoint custom resource is modeled after a standard Kubernetes resource
and is split into the `status` section:

```text
type SpiderEndpoint struct {
    [...]

    // Status is the status of the SpiderEndpoint
    Status WorkloadEndpointStatus `json:"status,omitempty"`
}
```

### SpiderEndpoint status

The `status` section contains some fields to describe details about the current Endpoint allocation.

```text
// WorkloadEndpointStatus defines the observed state of SpiderEndpoint
type WorkloadEndpointStatus struct {
    // the endpoint current allocation details
    Current *PodIPAllocation `json:"current,omitempty"`

    // the endpoint history allocation details
    History []PodIPAllocation `json:"history,omitempty"`

    // kubernetes controller owner reference
    OwnerControllerType string `json:"ownerControllerType"`
}

type PodIPAllocation struct {
    // container ID
    ContainerID string `json:"containerID"`

    // node name
    Node *string `json:"node,omitempty"`

    // allocated IPs
    IPs []IPAllocationDetail `json:"ips,omitempty"`

    // created time
    CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

type IPAllocationDetail struct {
    // interface name
    NIC string `json:"interface"`

    // IPv4 address
    IPv4 *string `json:"ipv4,omitempty"`

    // IPv6 address
    IPv6 *string `json:"ipv6,omitempty"`

    // IPv4 SpiderIPPool name
    IPv4Pool *string `json:"ipv4Pool,omitempty"`

    // IPv6 SpiderIPPool name
    IPv6Pool *string `json:"ipv6Pool,omitempty"`

    // vlan ID
    Vlan *int64 `json:"vlan,omitempty"`

    // IPv4 gateway
    IPv4Gateway *string `json:"ipv4Gateway,omitempty"`

    // IPv6 gateway
    IPv6Gateway *string `json:"ipv6Gateway,omitempty"`

    CleanGateway *bool `json:"cleanGateway,omitempty"`

    // route
    Routes []Route `json:"routes,omitempty"`
}
```
