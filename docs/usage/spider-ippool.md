# SpiderIPPool

**English** ｜ [**简体中文**](./spider-ippool-zh_CN.md)

## Introduction

SpiderIPPool resources represent the IP address ranges allocated by Spiderpool for Pods. To create SpiderIPPool resources in your cluster, refer to the [SpiderIPPool CRD](./../reference/crd-spiderippool.md).

## SpiderIPPool Features

- Single-stack, dual-stack, and IPv6 Support
- IP address range control
- Gateway route control
- Exclusive or global default pool control
- Compatible with various resource affinity settings

## Usage

### Single-stack and Dual-stack Control

Spiderpool supports three modes of IP address allocation: IPv4-only, IPv6-only, and dual-stack. Refer to [configmap](./../reference/configmap.md) for details.

> When installing Spiderpool via Helm, you can use configuration parameters to specify: `--set ipam.enableIPv4=true --set ipam.enableIPv6=true`.

If dual-stack mode is enabled, you can manually specify which IPv4 and IPv6 pools should be used for IP address allocation:

> In a dual-stack environment, you can also configure Pods to only receive IPv4 or IPv6 addresses using the annotation `ipam.spidernet.io/ippool: '{"ipv4": ["custom-ipv4-ippool"]}'`.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-dual-ippool-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: custom-dual-ippool-deploy
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["custom-ipv4-ippool"],"ipv6": ["custom-ipv6-ippool"]
          }
      labels:
        app: custom-dual-ippool-deploy
    spec:
      containers:
        - name: custom-dual-ippool-deploy
          image: busybox
          imagePullPolicy: IfNotPresent
          command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

### Specify IPPool to Allocate IP Addresses to Applications

This feature owns 4 usage options including: `Use Pod Annotation to Specify IP Pool`, `Use Namespace Annotation to Specify IP Pool`, `Use CNI Configuration File to Specify IP Pool` and `Set Cluster Default Level for SpiderIPPool`.

- For the priority rules when specifying the SpiderIPPool, refer to the [Candidate Pool Acquisition](../concepts/ipam-des.md#candidate-pool-acquisition).  
- Additionally, with the following ways of specifying IPPools(Pod Annotation, Namespace Annotation, CNI configuration file) you can also use wildcards '*', '?' and '[]' to match the desired IPPools. For example: ipam.spidernet.io/ippool: '{"ipv4": ["demo-v4-ippool1", "backup-ipv4*"]}'
  - '*': Matches zero or more characters. For example, "ab" can match "ab", "abc", "abcd", and so on.
  - '?': Matches a single character. For example, "a?c" can match "abc", "adc", "axc", and so on.
  - '[]': Matches a specified range of characters. You can specify the choices of characters inside the brackets, or use a hyphen to specify a character range. For example, "[abc]" can match any one of the characters "a", "b", or "c".

#### Use Pod Annotation to Specify IP Pool

You can use annotations like `ipam.spidernet.io/ippool` or `ipam.spidernet.io/ippools` on a Pod's annotation to indicate which IP pools should be used. The `ipam.spidernet.io/ippools` annotation is primarily used for specifying multiple network interfaces. Additionally, you can specify multiple pools as fallback options. If one pool's IP addresses are exhausted, addresses can be allocated from the other specified pools.

```yaml
ipam.spidernet.io/ippool: |-
  {
    "ipv4": ["demo-v4-ippool1", "backup-ipv4-ippool", "wildcard-v4?"],
    "ipv6": ["demo-v6-ippool1", "backup-ipv6-ippool", "wildcard-v6*"]
  }
```

When using the annotation `ipam.spidernet.io/ippools` for specifying multiple network interfaces, you can explicitly indicate the interface name by specifying the `interface` field. Alternatively, you can use **array ordering** to determine which IP pools are assigned to which network interfaces. Additionally, the `cleangateway` field indicates whether a default route should be generated based on the `gateway` field of the IPPool. When `cleangateway` is set to true, it means that no default route needs to be generated (default is false).

> In scenarios with multiple network interfaces, it is generally not possible to generate two or more default routes in the `main` routing table. The plugin `Coordinator` already solved this problem and you can ignore `clengateway` field. If you want to use Spiderpool IPAM plugin alone, you can use `cleangateway: true` to indicate that a default route should not be generated based on the IPPool `gateway` field.

```yaml
ipam.spidernet.io/ippools: |-
  [{
      "ipv4": ["demo-v4-ippool1", "wildcard-v4-ippool[123]"],
      "ipv6": ["demo-v6-ippool1", "wildcard-v6-ippool[123]"]
   },{
      "ipv4": ["demo-v4-ippool2", "wildcard-v4-ippool[456]"],
      "ipv6": ["demo-v6-ippool2", "wildcard-v6-ippool[456]"],
      "cleangateway": true
  }]
```

```yaml
ipam.spidernet.io/ippools: |-
  [{
      "interface": "eth0",
      "ipv4": ["demo-v4-ippool1", "wildcard-v4-ippool[123]"],
      "ipv6": ["demo-v6-ippool1", "wildcard-v6-ippool[123]"],
      "cleangateway": true
   },{
      "interface": "net1",
      "ipv4": ["demo-v4-ippool2", "wildcard-v4-ippool[456]"],
      "ipv6": ["demo-v6-ippool2", "wildcard-v6-ippool[456]"],
      "cleangateway": false
  }]
```

#### Use Namespace Annotation to Specify IP Pool

You can annotate the Namespace with `ipam.spidernet.io/default-ipv4-ippool` and `ipam.spidernet.io/default-ipv6-ippool`. When deploying applications, IP pools can be selected based on these annotations of the application's namespace:

> If IP pool is not explicitly specified, rules defined in the Namespace annotation take precedence.

```yaml

apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ipam.spidernet.io/default-ipv4-ippool: '["ns-v4-ippool1", "ns-v4-ippool2", "wildcard-v4*"]'
    ipam.spidernet.io/default-ipv6-ippool: '["ns-v6-ippool1", "ns-v6-ippool2", "wildcard-v6?"]'
  name: kube-system
...
```

#### Use CNI Configuration File to Specify IP Pool

You can specify the default IPv4 and IPv6 pools for an application in the CNI configuration file. For more details, refer to [CNI Configuration](./../reference/plugin-ipam.md)

> If IP pool is not explicitly specified using Pod Annotation and no IP pool is specified through Namespace annotation, the rules defined in the CNI configuration file take precedence.

```yaml
{
  "name": "macvlan-vlan0",
  "type": "macvlan",
  "master": "eth0",
  "ipam": {
    "type": "spiderpool",
    "default_ipv4_ippool":["default-v4-ippool", "backup-ipv4-ippool", "wildcard-v4-ippool[123]"],
    "default_ipv6_ippool":["default-v6-ippool", "backup-ipv6-ippool", "wildcard-v6-ippool[456]"]
    }
}
```

#### Set Cluster Default Level for SpiderIPPool

In the [SpiderIPPool CRD](./../reference/crd-spiderippool.md), the `spec.default` field is a boolean type. It determines the cluster default pool when no specific IPPool is specified through annotations or the CNI configuration file:

> - If no IP pool is specified using Pod annotations, and no IP pool is specified through Namespace annotations, and no IP pool is specified in the CNI configuration file, the system will use the pool defined by this field as the cluster default.
> - Multiple IPPool resources can be set as the cluster default level.

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: master-172
spec:
  default: true
...
```

### Use SpiderIPPool with Affinity

Refer to [SpiderIPPool Affinity](./spider-affinity.md) for details.

### SpiderIPPool Gateway and Route Configuration

Refer to [Route Support](./route.md) for details.

As a result, Pods will receive the default route based on the gateway, as well as custom routes defined in the IP pool. If the IP pool does not have a gateway configured, the default route will not take effect.

### View Extended Fields with Command Line (kubectl)

To simplify the viewing of properties related to SpiderIPPool resources, we have added some additional fields that can be displayed using the `kubectl get sp -o wide` command:

- `ALLOCATED-IP-COUNT` represents the number of allocated IP addresses in the pool.
- `TOTAL-IP-COUNT` represents the total number of IP addresses in the pool.
- `DEFAULT` indicates whether the pool is set as the cluster default level.
- `DISABLE` indicates whether the pool is disabled.
- `NODENAME` indicates the nodes have an affinity with the pool.
- `MULTUSNAME` indicates the Multus instances have an affinity with the pool.
- `APP-NAMESPACE` is specific to the [SpiderSubnet](./spider-subnet.md) feature. It signifies that the pool is automatically created by the system and corresponds to the namespace of the associated application.

```shell
~# kubectl get sp -o wide  
NAME                                  VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE   NODENAME               MULTUSNAME                      APP-NAMESPACE
auto4-demo-deploy-subnet-eth0-fcca4   4         172.100.0.0/16            1                    2                false     false                                                            kube-system
test-pod-ippool                       4         10.6.0.0/16               0                    10               false     false     ["master","worker1"]   ["kube-system/macvlan-vlan0"]   
```

### Metrics

We have also supplemented SpiderIPPool resources with relevant metric information. For more details, refer to [metric](./../reference/metrics.md)
