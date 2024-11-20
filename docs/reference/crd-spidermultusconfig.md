# SpiderMultusConfig

A SpiderMultusConfig resource represents a best practice to generate a multus net-attach-def CR object for spiderpool to use.

For details on using this CRD, please read the [SpiderMultusConfig guide](./../usage/spider-multus-config.md).

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: demo
  namespace: default
  annotations:
    multus.spidernet.io/cr-name: "macvlan-100"
    multus.spidernet.io/cni-version: 0.4.0
spec:
  cniType: macvlan
  macvlan:
    master: ["eth0"]
    vlanID: 100
    ippools:
      ipv4: ["default-pool-v4"]
      ipv6: ["default-pool-v6"]
```

## SpiderMultusConfig definition

### Metadata

| Field       | Description                                         | Schema | Validation |
|-------------|-----------------------------------------------------|--------|------------|
| name        | The name of this SpiderMultusConfig resource        | string | required   |
| namespace   | The namespace of this SpiderMultusConfig resource   | string | required   |
| annotations | The annotations of this SpiderMultusConfig resource | map    | optional   |

### Metadata.annotations

You can also set annotations for this SpiderMultusConfig resource, then the corresponding Multus net-attach-def resource will inherit these annotations too.  
And you can also use special annotation `multus.spidernet.io/cr-name` and `multus.spidernet.io/cni-version` to customize the corresponding Multus net-attach-def resource name and CNI version.

| Field                           | Description                                               | Schema | Validation | Default |
|---------------------------------|-----------------------------------------------------------|--------|------------|---------|
| multus.spidernet.io/cr-name     | The customized Multus net-attach-def resource name        | string | optional   |         |
| multus.spidernet.io/cni-version | The customized Multus net-attach-def resource CNI version | string | optional   | 0.3.1   |

### Spec

This is the SpiderReservedIP spec for users to configure.

| Field             | Description                                                                                 | Schema                                                                       | Validation | Values                                        | Default |
|-------------------|---------------------------------------------------------------------------------------------|------------------------------------------------------------------------------|------------|-----------------------------------------------|---------|
| cniType           | expected main CNI type                                                                      | string                                                                       | require    | macvlan, ipvlan, sriov, ovs, ib-sriov, custom |         |
| macvlan           | macvlan CNI configuration                                                                   | [SpiderMacvlanCniConfig](./crd-spidermultusconfig.md#spidermacvlancniconfig) | optional   |                                               |         |
| ipvlan            | ipvlan CNI configuration                                                                    | [SpiderIPvlanCniConfig](./crd-spidermultusconfig.md#spideripvlancniconfig)   | optional   |                                               |         |
| sriov             | sriov CNI configuration                                                                     | [SpiderSRIOVCniConfig](./crd-spidermultusconfig.md#spidersriovcniconfig)     | optional   |                                               |         |
| ibsriov           | infiniband ib-sriov CNI configuration                                                       | [SpiderIBSRIOVCniConfig](./crd-spidermultusconfig.md#spideribsriovcniconfig) | optional   |                                               |         |
| ipoib             | infiniband ipoib CNI configuration                                                          | [SpiderIpoibCniConfig](./crd-spidermultusconfig.md#spideripoibcniconfig)     | optional   |                                               |         |
| ovs               | ovs CNI configuration                                                                       | [SpiderOvsCniConfig](./crd-spidermultusconfig.md#spiderovscniconfig)         | optional   |                                               |         |
| enableCoordinator | enable coordinator or not                                                                   | boolean                                                                      | optional   | true,false                                    | true    |
| disableIPAM       | disable IPAM. when set to be true, any configuration of CNI's ippools field will be ignored | boolean                                                                      | optional   | true,false                                    | false    |
| coordinator       | coordinator CNI configuration                                                               | [CoordinatorSpec](./crd-spidercoordinator.md#spec)                           | optional   |                                               |         |
| customCNI         | a string that represents custom CNI configuration                                           | string                                                                       | optional   |                                               |         |
| chainCNIJsonData         | a list of string that represents chain CNI configuration, such as tune plugin.                                           | []string                                                                       | optional   |                                               |         |

#### SpiderMacvlanCniConfig

| Field   | Description                                                                                                                        | Schema                                                         | Validation | Values   |
|---------|------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------|------------|----------|
| master  | the Interfaces on your master, you could specify a single one Interface<br/> or multiple Interfaces to generate one bond Interface | list of strings                                                | required   |          |
| vlanID  | vlan ID                                                                                                                            | int                                                            | optional   | [0,4094] |
| bond    | expected bond Interface configurations                                                                                             | [BondConfig](./crd-spidermultusconfig.md#bondconfig)           | optional   |          |
| rdmaResourceName | rdma resource name of the spiderMultusConfig, it often reported to kubelet by the k8s-rdma-shared-dev-plugin. when it is not empty and spiderpool podResourceInject feature is enabled, spiderpool can automatically inject it into the container's resources via webhook                                                                                                                      | string                                                         | optional   |          |
| ippools | the default IPPools in your CNI configurations                                                                                     | [SpiderpoolPools](./crd-spidermultusconfig.md#spiderpoolpools) | optional   |          |

#### SpiderIPvlanCniConfig

| Field   | Description                                                                                                                        | Schema                                                         | Validation | Values   |
|---------|------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------|------------|----------|
| master  | the Interfaces on your master, you could specify a single one Interface<br/> or multiple Interfaces to generate one bond Interface | list of strings                                                | required   |          |
| vlanID  | vlan ID                                                                                                                            | int                                                            | optional   | [0,4094] |
| bond    | expected bond Interface configurations                                                                                             | [BondConfig](./crd-spidermultusconfig.md#bondconfig)           | optional   |          |
| rdmaResourceName | rdma resource name of the spiderMultusConfig, it often reported to kubelet by the k8s-rdma-shared-dev-plugin. when it is not empty and spiderpool podResourceInject feature is enabled, spiderpool can automatically inject it into the container's resources via webhook                                                                                                                      | string                                                         | optional   |          |
| ippools | the default IPPools in your CNI configurations                                                                                     | [SpiderpoolPools](./crd-spidermultusconfig.md#spiderpoolpools) | optional   |          |

#### SpiderSRIOVCniConfig

| Field         | Description                                                                               | Schema                                                         | Validation |
|---------------|-------------------------------------------------------------------------------------------|----------------------------------------------------------------|------------|
| resourceName  | this property will create an annotation for Multus net-attach-def to cooperate with SRIOV, if spiderpool podResourceInject feature is enabled, spiderpool can automatically inject it into the container's resources via webhook | string                                                         | required   |
| vlanID        | vlan ID                                                                                   | int                                                            | optional   |
| minTxRateMbps | change the allowed minimum transmit bandwidth, in Mbps, for the VF. Setting this to 0 disables rate limiting. The min_tx_rate value should be <= max_tx_rate. Support of this feature depends on NICs and drivers | int | optional |
| maxTxRateMbps | change the allowed maximum transmit bandwidth, in Mbps, for the VF. Setting this to 0 disables rate limiting | int | optional |
| enableRdma(deprecated)    | It will be remove in the furture, use     rdmaIsolation instead.                                  | bool                                                           | optional   |
 rdmaIsolation  | rdmaIsolation enable RDMA CNI plugin is intended to be run as a chained CNI plugin. it ensures isolation of RDMA traffic from other workloads in the system by moving the associated RDMA interfaces of the provided network interface to the container's network namespace path.                                                                                                                                                                                                                       | bool                                                           | optional   |
| ippools       | the default IPPools in your CNI configurations                                            | [SpiderpoolPools](./crd-spidermultusconfig.md#spiderpoolpools) | optional   |

#### SpiderIBSRIOVCniConfig

| Field                | Description                                                                                                                                                                                                                                     | Schema                                                         | Validation |
|----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------|------------|
| resourceName         | this property will create an annotation for Multus net-attach-def to cooperate with SRIOV, if spiderpool podResourceInject feature is enabled, spiderpool can automatically inject it into the container's resources via webhook                                                                                                                                                    | string                                                         | required   |
| pkey                 | InfiniBand pkey for VF, this field is used by [ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) to add pkey with guid to InfiniBand subnet manager client e.g. [Mellanox UFM](https://www.nvidia.com/en-us/networking/infiniband/ufm/) | string                                                         | optional   |
| linkState            | Enforces link state for the VF. Allowed values: auto, enable [default], disable                                                                                                                                                                 | string                                                         | optional   |
| rdmaIsolation        | Enable RDMA network namespace isolation for RDMA workloads, default to true                                                                                                                                                                     | bool                                                           | optional   |
| ibKubernetesEnabled  | Enforces ib-sriov-cni to work with [ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) , default to false                                                                                                                                | bool                                                           | optional   |
| ippools              | the default IPPools in your CNI configurations                                                                                                                                                                                                  | [SpiderpoolPools](./crd-spidermultusconfig.md#spiderpoolpools) | optional   |

#### SpiderIpoibCniConfig

| Field   | Description                                   | Schema                                                         | Validation |
|---------|-----------------------------------------------|----------------------------------------------------------------|------------|
| master  | master interface name                         | string                                                         | required   |
| ippools | the default IPPools in your CNI configurations | [SpiderpoolPools](./crd-spidermultusconfig.md#spiderpoolpools) | optional   |

#### SpiderOvsCniConfig

| Field        | Description                                                                               | Schema                                                         | Validation |
|--------------|-------------------------------------------------------------------------------------------|----------------------------------------------------------------|------------|
| bridge       | name of the bridge to use                                                                 | string                                                         | required   |
| vlan         | vlan ID of attached port. Trunk port if not specified                                     | int                                                            | optional   |
| trunk        | List of VLAN ID's and/or ranges of accepted VLAN ID's                                     | [Trunk](./crd-spidermultusconfig.md#trunk)                     | optional   |
| deviceID     | PCI address of a VF in valid sysfs format                                                 | string                                                         | optional   |
| ippools      | the default IPPools in your CNI configurations                                            | [SpiderpoolPools](./crd-spidermultusconfig.md#spiderpoolpools) | optional   |

#### BondConfig

| Field                 | Description                            | Schema | Validation | Values |
|-----------------------|----------------------------------------|--------|------------|--------|
| Name                  | the expected bond interface name       | string | required   |        |
| Mode                  | bond interface mode                    | int    | required   | [0,6]  |
| Options               | expected bond Interface configurations | string | optional   |        |

#### Trunk

| Field                 | Description                            | Schema | Validation | Values   |
|-----------------------|----------------------------------------|--------|------------|----------|
| minID                 | the min value of vlan ID               | int    | optional   | [0,4094] |
| maxID                 | the max value of vlan ID               | int    | optional   | [0,4094] |
| id                    | the value of vlan ID                   | int    | optional   | [0,4094] |

#### SpiderpoolPools

| Field | Description                                         | Schema          | Validation |
|-------|-----------------------------------------------------|-----------------|------------|
| ipv4  | the default IPv4 IPPools in your CNI configurations | list of strings | optional   |
| ipv6  | the default IPv6 IPPools in your CNI configurations | list of strings | optional   |
