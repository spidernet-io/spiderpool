# SpiderMultusConfig

A SpiderMultusConfig resource represents a best practice to generate a multus net-attach-def CR object for spiderpool to use.

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-100
  namespace: default
spec:
  cniType: macvlan
  macvlan:
    master: ["eth0"]
    vlanID: 100
    ippools:
      ipv4: ["default-pool-v4"]
      ipv6: ["default-pool-v6"]
```

## SpiderReservedIP definition

### Metadata

| Field     | Description                                       | Schema | Validation |
|-----------|---------------------------------------------------|--------|------------|
| name      | The name of this SpiderMultusConfig resource      | string | required   |
| namespace | The namespace of this SpiderMultusConfig resource | string | required   |


### Spec

This is the SpiderReservedIP spec for users to configure.

| Field             | Description                                       | Schema                                                                   | Validation | Values                      | Default |
|-------------------|---------------------------------------------------|--------------------------------------------------------------------------|------------|-----------------------------|---------|
| cniType           | expected main CNI type                            | string                                                                   | require    | macvlan,ipvlan,sriov,custom |         |
| macvlan           | macvlan CNI configuration                         | [SpiderMacvlanCniConfig](./spidermultusconfig.md#SpiderMacvlanCniConfig) | optional   |                             |         |
| ipvlan            | ipvlan CNI configuration                          | [SpiderIPvlanCniConfig](./spidermultusconfig.md#SpiderIPvlanCniConfig)   | optional   |                             |         |
| sriov             | sriov CNI configuration                           | [SpiderSRIOVCniConfig](./spidermultusconfig.md#SpiderSRIOVCniConfig)     | optional   |                             |         |
| enableCoordinator | enable coordinator or not                         | boolean                                                                  | optional   | true,false                  | true    |
| coordinator       | coordinator CNI configuration                     | CoordinatorSpec                                                          | optional   |                             |         |
| customCNI         | a string that represents custom CNI configuration | string                                                                   | optional   |                             |         |

#### SpiderMacvlanCniConfig

| Field   | Description                                                                                                                        | Schema                                                     | Validation | Values   |
|---------|------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|------------|----------|
| master  | the Interfaces on your master, you could specify a single one Interface<br/> or multiple Interfaces to generate one bond Interface | list of strings                                            | required   |          |
| vlanID  | vlan ID                                                                                                                            | int                                                        | optional   | [0,4095] |
| bond    | expected bond Interface configurations                                                                                             | [BondConfig](./spidermultusconfig.md#BondConfig)           | optional   |          |
| ippools | the default IPPools in your CNI configurations                                                                                     | [SpiderpoolPools](./spidermultusconfig.md#SpiderpoolPools) | optional   |          |

#### SpiderIPvlanCniConfig

| Field   | Description                                                                                                                        | Schema                                                     | Validation | Values   |
|---------|------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|------------|----------|
| master  | the Interfaces on your master, you could specify a single one Interface<br/> or multiple Interfaces to generate one bond Interface | list of strings                                            | required   |          |
| vlanID  | vlan ID                                                                                                                            | int                                                        | optional   | [0,4095] |
| bond    | expected bond Interface configurations                                                                                             | [BondConfig](./spidermultusconfig.md#BondConfig)           | optional   |          |
| ippools | the default IPPools in your CNI configurations                                                                                     | [SpiderpoolPools](./spidermultusconfig.md#SpiderpoolPools) | optional   |          |

#### SpiderSRIOVCniConfig

| Field        | Description                                                                               | Schema                                                     | Validation |
|--------------|-------------------------------------------------------------------------------------------|------------------------------------------------------------|------------|
| resourceName | this property will create an annotation for Multus net-attach-def to cooperate with SRIOV | string                                                     | required   |
| vlanID       | vlan ID                                                                                   | int                                                        | optional   |
| ippools      | the default IPPools in your CNI configurations                                            | [SpiderpoolPools](./spidermultusconfig.md#SpiderpoolPools) | optional   |

#### BondConfig

| Field                 | Description                            | Schema | Validation | Values |
|-----------------------|----------------------------------------|--------|------------|--------|
| Name                  | the expected bond interface name       | string | required   |        |
| Mode                  | bond interface mode                    | int    | required   | [0,6]  |
| Options               | expected bond Interface configurations | string | optional   |        |

#### SpiderpoolPools

| Field | Description                                         | Schema          | Validation |
|-------|-----------------------------------------------------|-----------------|------------|
| ipv4  | the default IPv4 IPPools in your CNI configurations | list of strings | optional   |
| ipv6  | the default IPv6 IPPools in your CNI configurations | list of strings | optional   |
