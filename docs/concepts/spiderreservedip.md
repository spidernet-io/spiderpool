# SpiderReservedIP

A SpiderReservedIP resource represents a collection of IP addresses that Spiderpool expects not to be allocated.

## CRD definition

The SpiderReservedIP custom resource is modeled after a standard Kubernetes resource
and is split into a `spec` section:

```text
// SpiderReservedIP is the Schema for the spiderreservedips API
type SpiderReservedIP struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec ReservedIPSpec `json:"spec,omitempty"`
}
```

### SpiderReservedIP spec

The `spec` section embeds a specific ReservedIP field which allows to define the list of all reserved IPs:

```text
// ReservedIPSpec defines the desired state of SpiderReservedIP
type ReservedIPSpec struct {
    // IP version
    IPVersion *int64 `json:"ipVersion,omitempty"`
ni
    // reserved IPs
    IPs []string `json:"ips"`
}
```
