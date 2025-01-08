# SpiderSubnet

A SpiderSubnet resource represents a collection of IP addresses from which Spiderpool expects SpiderIPPool IPs to be assigned.

For details on using this CRD, please read the [SpiderSubnet guide](./../usage/spider-subnet.md).

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: default-v4-subnet
spec:
  ipVersion: 4
  ips:
    - 172.22.40.2-172.22.40.254
  subnet: 172.22.0.0/16
  excludeIPs:
    - 172.22.40.10-172.22.40.20
  gateway: 172.22.40.1
```

## SpiderSubnet definition

### Metadata

| Field | Description                            | Schema | Validation |
|-------|----------------------------------------|--------|------------|
| name  | the name of this SpiderSubnet resource | string | required   |

### Spec

This is the SpiderSubnet spec for users to configure.

| Field             | Description                                    | Schema                                       | Validation | Values                                   | Default |
|-------------------|------------------------------------------------|----------------------------------------------|------------|------------------------------------------|---------|
| ipVersion         | IP version of this subnet                      | int                                          | optional   | 4,6                                      |         |
| subnet            | subnet of this resource                        | string                                       | required   | IPv4 or IPv6 CIDR.<br/>Must not overlap  |         |
| ips               | IP ranges for this resource to use             | list of strings                              | optional   | array of IP ranges and single IP address |         |
| excludeIPs        | isolated IP ranges for this resource to filter | list of strings                              | optional   | array of IP ranges and single IP address |         |
| gateway           | gateway for this resource                      | string                                       | optional   | an IP address                            |         |
| routes            | custom routes in this resource                 | list of [Route](./crd-spiderippool.md#route) | optional   |                                          |         |

### Status (subresource)

The Subnet status is a subresource that processed automatically by the system to summarize the current state.

| Field             | Description                                              | Schema |
|-------------------|----------------------------------------------------------|--------|
| controlledIPPools | current IP allocations in this subnet resource           | string |
| totalIPCount      | total IP addresses counts of this subnet resource to use | int    |
| allocatedIPCount  | current allocated IP addresses counts                    | int    |
