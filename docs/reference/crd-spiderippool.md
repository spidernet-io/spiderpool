# SpiderIPPool

A SpiderIPPool resource represents a collection of IP addresses from which Spiderpool expects endpoint IPs to be assigned.

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: master-172
spec:
  ipVersion: 4
  subnet: 172.31.192.0/20
  ips:
    - 172.31.199.180-172.31.199.189
    - 172.31.199.205-172.31.199.209
  excludeIPs:
    - 172.31.199.186-172.31.199.188
    - 172.31.199.207
  gateway: 172.31.207.253
  default: true
  disable: false
```

## SpiderIPPool definition

### Metadata

| Field | Description                            | Schema | Validation |
|-------|----------------------------------------|--------|------------|
| name  | the name of this SpiderIPPool resource | string | required   |

### Spec

This is the IPPool spec for users to configure.

| Field             | Description                                                                                                | Schema                                                                                                                                 | Validation | Values                                   | Default |
|-------------------|------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------|------------|------------------------------------------|---------|
| ipVersion         | IP version of this pool                                                                                    | int                                                                                                                                    | optional   | 4,6                                      |         |
| subnet            | subnet of this pool                                                                                        | string                                                                                                                                 | required   | IPv4 or IPv6 CIDR.<br/>Must not overlap  |         |
| ips               | IP ranges for this pool to use                                                                             | list of strings                                                                                                                        | optional   | array of IP ranges and single IP address |         |
| excludeIPs        | isolated IP ranges for this pool to filter                                                                 | list of strings                                                                                                                        | optional   | array of IP ranges and single IP address |         |
| gateway           | gateway for this pool                                                                                      | string                                                                                                                                 | optional   | an IP address                            |         |
| routes            | custom routes in this pool (please don't set default route `0.0.0.0/0` if property `gateway` exists)       | list of [route](./crd-spiderippool.md#route)                                                                                           | optional   |                                          |         |
| podAffinity       | specify which pods can use this pool                                                                       | [labelSelector](https://github.com/kubernetes/kubernetes/blob/v1.27.0/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/types.go#L1195) | optional   | kubernetes LabelSelector                 |         |
| namespaceAffinity | specify which namespaces pods can use this pool                                                            | [labelSelector](https://github.com/kubernetes/kubernetes/blob/v1.27.0/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/types.go#L1195) | optional   | kubernetes LabelSelector                 |         |
| namespaceName     | specify which namespaces pods can use this pool (The priority is higher than property `namespaceAffinity`) | list of strings                                                                                                                        | optional   |                                          |         |
| nodeAffinity      | specify which nodes pods can use this pool                                                                 | [labelSelector](https://github.com/kubernetes/kubernetes/blob/v1.27.0/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/types.go#L1195) | optional   | kubernetes LabelSelector                 |         |
| nodeName          | specify which nodes pods can use this pool (The priority is higher than property `nodeAffinity`)           | list of strings                                                                                                                        | optional   |                                          |         |
| multusName        | specify which multus net-attach-def objects can use this pool                                              | list of strings                                                                                                                        | optional   |                                          |         |
| default           | configure this resource as a default pool for pods                                                         | boolean                                                                                                                                | optional   | true,false                               | false   |
| disable           | configure whether the pool is usable                                                                       | boolean                                                                                                                                | optional   | true,false                               | false   |

### Status (subresource)

The IPPool status is a subresource that processed automatically by the system to summarize the current state

| Field             | Description                         | Schema |
|-------------------|-------------------------------------|--------|
| allocatedIPs      | current IP allocations in this pool | string |
| totalIPCount      | total IP counts of this pool to use | int    |
| allocatedIPCount  | current allocated IP counts         | int    |

#### Route

| Field | Description               | Schema | Validation  |
|-------|---------------------------|--------|-------------|
| dst   | destination of this route | string | required    |
| gw    | gateway of this route     | string | required    |

### Pod Affinity

For details on configuring SpiderIPPool podAffinity, please read the [Pod Affinity of IPPool](../usage/spider-affinity.md).

### Namespace Affinity

For details on configuring SpiderIPPool namespaceAffinity or namespaceName, please read the [Namespace Affinity of IPPool](../usage/spider-affinity.md).
> Notice: `namespaceName` has higher priority than `namespaceAffinity`.

### Node Affinity

For details on configuring SpiderIPPool nodeAffinity or nodeName, please read the [Node Affinity of IPPool](../usage/spider-affinity.md) and [Network topology allocation](./../usage/network-topology.md).
> Notice: `nodeName` has higher priority than `nodeAffinity`.

### Multus Affinity

For details on configuring SpiderIPPool multusName, please read the [multus Affinity of IPPool](../usage/spider-affinity.md).
