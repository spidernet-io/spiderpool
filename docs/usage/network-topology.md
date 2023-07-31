# 基于节点网络区域的IP分配

对于一些基础设施有特殊需求的用户，例如集群的节点是跨网段一个节点对应一个子网的场景，我们的SpiderPool也可以轻松应对。

![network-topology](../images/network-topology.png)

## Spiderpool相关功能介绍

### 多池备选

spiderpool在分配IP的时候，可支持多池备选功能，一个Pod不仅仅只能从某个具体的IPPool中分配出一个IP。该功能设计的意义是当使用某个池来为一组Pod分配IP时，若第一个IPPool的IP不够分，那么可顺序的从后续的备选IPPool中分出IP。

+ 使用annotation来指定IPPools，可给Pod打上 `ipam.spidernet.io/ippool` 或 `ipam.spidernet.io/ippools` 的annotation来指定IPPool，详情可见 [Pod Annotation](../reference/annotation.md)

+ 使用CNI配置文件来指定IPPools，可在 `default_ipv4_ippool` 和 `default_ipv6_ippool` 中设置期望使用的IPPools，详情可见 [configuration](../reference/plugin-ipam.md)

+ 设置缺省IPPool, 可给 SpiderIPPool CR实例设置 `default`来指定一系列的缺省IP池，详情可见SpiderIPPool的CRD定义 [SpiderIPPool](../reference/crd-spiderippool.md)

### IPPool亲和性设置

一个IP池可以根据亲和性绑定给某个Node，这意味着当一个Pod指定使用多个拥有nodeAffinity的IP池的时候，spiderpool只会使用与Pod当前所在节点亲和的IP池来分配IP。 详情可见 [IPPool-NodeAffinity](../usage/ippool-affinity-node.md)

## 场景示例

例如在IPv4的单栈集群中拥有两个节点分别名为master和worker，并且限制每个节点仅仅只能使用对应的IPPool(pool-master,pool-worker)。在此场景下，可使用以上 "多池备选" 篇章中介绍的3种方案实现。

第一步，先给Node以及对应的IPPool设置亲和绑定来强制限定某些池仅能使用于哪些节点。

第二步，选择一个方案来指定多池备选。

+ 例如给Pod指定Annotation

```text
ipam.spidernet.io/ippool: |-
  {
    "ipv4": ["pool-master", "pool-worker"],
  }
```

+ 例如设置好CNI配置文件，此场景下就无需为Pod打上Annotation

```text
{
    "cniVersion": "0.3.1",
    "name": "mynet",
    "type": "macvlan",
    "master": "eth0",
    "ipam":{
        "type":"spiderpool",
        "default_ipv4_ippool": ["pool-master","pool-worker"],
    }
}
```

+ 例如给IPPool的CR实例设置default字段作为缺省池使用，此场景下就无需为Pod打上Annotation或者去修改CNI配置文件

```text
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-master
spec:
  default: true
  ips:
  - 10.125.177.2-10.125.177.61
  subnet: 10.125.177.0/24
  nodeAffinity:
    matchLabels:
      role: master
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-worker
spec:
  default: true
  ips:
  - 172.22.40.2-172.22.40.254
  subnet: 172.22.0.0/16
  nodeAffinity:
    matchLabels:
      role: worker
```
