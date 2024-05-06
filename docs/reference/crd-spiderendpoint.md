# SpiderEndpoint

A SpiderEndpoint resource represents IP address allocation details for the corresponding pod. This resource one to one pod, and it will inherit the pod name and pod namespace.

> Notice: For kubevirt VM static IP feature, the SpiderEndpoint object would inherit the kubevirt VM/VMI resource name and namespace.

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderEndpoint
metadata:
  name: test-app-1-9dc78fb9-rs99d
status:
  current:
    ips:
    - cleanGateway: false
      interface: eth0
      ipv4: 172.31.199.193/20
      ipv4Gateway: 172.31.207.253
      ipv4Pool: worker-172
      vlan: 0
    node: dc-test02
    uid: e7b50a38-25c2-41d0-b332-7f619c69194e
  ownerControllerName: test-app-1
  ownerControllerType: Deployment
```

## SpiderEndpoint definition

### Metadata

| Field     | Description                                   | Schema | Validation |
|-----------|-----------------------------------------------|--------|------------|
| name      | the name of this SpiderEndpoint resource      | string | required   |
| namespace | the namespace of this SpiderEndpoint resource | string | required   |

### Status (subresource)

The IPPool status is a subresource that processed automatically by the system to summarize the current state.

| Field               | Description                                        | Schema                                                     | Validation |
|---------------------|----------------------------------------------------|------------------------------------------------------------|------------|
| current             | the IP allocation details of the corresponding pod | [PodIPAllocation](./crd-spiderendpoint.md#podipallocation) | required   |
| ownerControllerType | the corresponding pod top owner controller type    | string                                                     | required   |
| ownerControllerName | the corresponding pod top owner controller name    | string                                                     | required   |

#### PodIPAllocation

This property describes the SpiderEndpoint corresponding pod details.

| Field | Description                         | Schema                                                                   | Validation |
|-------|-------------------------------------|--------------------------------------------------------------------------|------------|
| uid   | corresponding pod uid               | string                                                                   | required   |
| node  | total IP counts of this pool to use | string                                                                   | required   |
| ips   | current allocated IP counts         | list of [IPAllocationDetail](./crd-spiderendpoint.md#podipallocation) | required   |

#### IPAllocationDetail

This property describes single Interface allocation details.

| Field        | Description                                                | Schema                                       | Validation | Default |
|--------------|------------------------------------------------------------|----------------------------------------------|------------|---------|
| interface    | single interface name                                      | string                                       | required   |         |
| ipv4         | single IPv4 allocated IP address                           | string                                       | optional   |         |
| ipv6         | single IPv6 allocated IP address                           | string                                       | optional   |         |
| ipv4Pool     | the IPv4 allocated IP address corresponding pool           | string                                       | optional   |         |
| ipv6Pool     | the IPv6 allocated IP address corresponding pool           | string                                       | optional   |         |
| vlan         | vlan ID                                                    | int                                          | optional   | 0       |
| ipv4Gateway  | the IPv4 gateway IP address                                | string                                       | optional   |         |
| ipv6Gateway  | the IPv6 gateway IP address                                | string                                       | optional   |         |
| cleanGateway | a flag to choose whether need default route by the gateway | boolean                                      | optional   |         |
| routes       | the allocation routes                                      | list if [Route](./crd-spiderippool.md#route) | optional   |         |
