# Spiderpool IPPool Selection Rules

**简体中文** | [**English**](./spider-allocation.md)

## 介绍

Spiderpool 允许动态分配和管理 IP 地址。在为 Pod 分配 IP 地址时，会严格遵守 **高优先级到低优先级** 的池选择规则。

## IPPool 选择规则

以下规则按照从 **高优先级到低优先级** 的顺序列出，如果同时存在下面的多个规则，前一个规则将 **覆盖** 后一个规则。

### SpiderSubnet 注解

SpiderSubnet 资源代表 IP 地址的集合，当需要为应用分配固定的 IP 地址时，应用管理员需要平台管理员告知可用的 IP 地址和路由属性等，但双方分属两个不同的运营部门，这使得每一个应用创建的工作流程繁琐，借助于 Spiderpool 的 SpiderSubnet 功能，它能自动从中子网分配 IP 给 IPPool，并且还能为应用固定 IP 地址，极大的减少了运维的成本。

SpiderSubnet 功能默认关闭，通过 Helm 部署时，可以通过 `--set ipam.enableSpiderSubnet=true` 开启 ，该功能开启后，创建应用时可以使用 `ipam.spidernet.io/subnets` 或 `ipam.spidernet.io/subnet` 注解指定 Subnet，从而实现从子网中随机选取 IP 地址自动建池，并分配固定 IP 地址给应用。有关详情，请参阅 [SpiderSubnet](./spider-subnet-zh_CN.md)。

NOTE:

> `ipam.spidernet.io/subnets` 的优先级是大于 `ipam.spidernet.io/subnet` 的。

### SpiderIPPool 注解

SpiderIPPool 资源代表 IP 地址的集合，一个 Subnet 中的不同 IP 地址，可分别存储到不同的 IPPool 实例中（Spiderpool 会校验 IPPool 之间的地址集合不重叠）。依据需求，SpiderIPPool 中的 IP 集合可大可小。能很好的应对 Underlay 网络的 IP 地址资源有限情况，且这种设计特点，创建应用时，结合 SpiderIPPool 注解能绑定不同的 IPPool，也能分享相同的 IPPool，既能够让所有应用共享使用同一个 Subnet，又能够实现 "微隔离"。

当创建应用时可以使用 `ipam.spidernet.io/ippools` 或 `ipam.spidernet.io/ippool` 注解指定 IPPool，实现从特定池中分配 IP 地址，前者的优先级大于后者。该方式是 Spiderpool 最推荐的用法，在绝大部分场景中，你将会使用到它们。

### 租户默认 IP 池

管理员往往会在集群划分多租户，能更好地隔离、管理和协作，同时也能提供更高的安全性、资源利用率和灵活性等。因此，会将不同功能的应用部署在不同租户下，在相同租户下的应用共享一个池或多个池，或者多个租户共享一个池或多个池，以此，能帮助管理员减少运维负担，创建应用时只需关注所属租户即可。

Spiderpool 的租户默认 IP 池功能即可满足上述需求，在租户中通过设置注解 `ipam.spidernet.io/default-ipv4-ippool` 指定默认的 IP 池，如果是双栈环境，需同时设置 `ipam.spidernet.io/default-ipv6-ippool`。在该租户中创建应用时，如果没有其他高优先级的池规则，那么将从该租户可用的候选池中尝试分配 IP 地址。

### CNI 配置文件

一个拥有成百上千节点的超大规模集群，但集群的节点分布在不同地区或数据中心，一些节点的区域只能使用子网 10.6.1.0/24，一些节点的区域只能使用子网 172.16.2.0/24 等。在该场景中，当创建应用时，以保证应用的高可靠性，应用的副本期望需要分布到不同的节点。因此 IPAM 需要为应用的每个副本分配不同的子网的 IP 地址。

Spiderpool 提供了两种方式解决上述问题。

1. 通过 Spiderpool 的 SpiderIPPool CR 中提供的 `nodeName` 或者 `nodeAffinity` 字段，设置 IP 池与节点之间的亲和。然后创建应用时，通过使用 `ipam.spidernet.io/ippools` 或 `ipam.spidernet.io/ippool` 注解指定多个 IP 池，但是该方式，在每次创建应用时，都需要使用 IPPool 注解指定多个池，在超大规模集群中，这将导致注解非常的冗长，某个池名写错或者写漏，难以排查。运维会非常困难。

2. 通过在 CNI 配置文件中的 "default_ipv4_ippool" 和 "default_ipv6_ippool" 字段设置全局的 CNI 默认池，其可以设置多个 IP 池用作备选池，当应用使用该 CNI 配置网络时并调用 Spiderpool ，对于每个应用副本，Spiderpool 都会按照 "IP 池数组" 中元素的顺序依次尝试分配 IP 地址，在每个节点分属不同的地区或数据中心的场景，如果应用副本被调度到的节点，符合第一个 IP 池的节点亲和规则，Pod 会从该池中获得 IP 分配，如果不满足，Spiderpool 会尝试从备选池中选择 IP 池继续为 Pod 分配 IP ，直到所有备选池全部筛选失败。详细信息请参考[CNI 配置](../reference/plugin-ipam.md)。

对于超大集群，集群节点分属于不同数据中心的场景下，推荐使用方式 2，能避免每次创建应用，繁琐的书写池注解。

### 集群默认 IPPool

为了节省运维成本，且无其他特殊需求。在创建 SpiderIPPool CR 对象时通过 **spec.default: true** 将该池设置为集群默认 IPPool，如果上述多次选池规则不存在时，将使用集群默认的 IPPool 为应用分配 IP 地址。

## 总结

本章节中介绍了 Spiderpool 为应用分配 IP 地址时的选池规则，Spiderpool 会严格遵守 **高优先级到低优先级** 的池选择规则。
