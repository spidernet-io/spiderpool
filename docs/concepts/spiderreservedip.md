# SpiderReservedIP

A SpiderReservedIP resource represents a collection of IP addresses that Spiderpool expects not to be allocated.

## CRD Definition

The SpiderReservedIP custom resource is modeled after a standard Kubernetes resource
and is split into a ``spec`` section:

```text
// SpiderReservedIP is the Schema for the spiderreservedips API
type SpiderReservedIP struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec ReservedIPSpec `json:"spec,omitempty"`
}
```

### SpiderReservedIP Specification

The ``spec`` section embeds an ReservedIP specific field which allows to define the list of all reserved IPs:

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
