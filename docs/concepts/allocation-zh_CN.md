# IP Allocation

**简体中文** | [**English**](./allocation.md)

当 Pod 创建时，它将按照以下步骤获取 IP 分配；IP 分配生命周期将经历 `获取候选池`、`过滤候选池`、`候选池排序` 三个大阶段。

- 获取候选池：Spiderpool 有多种池选择规则，会严格遵守 **高优先级到低优先级** 的池选择规则，获取**高优先级规则**命中的所有池，将它们标记为候选者身份，以有资格被进一步考虑。

- 过滤候选池：Spiderpool 通过亲和性等过滤机制，更精确地从所有候选池中选择合适的候选池，以满足特定的需求或复杂的使用场景。

- 候选池排序：对于多候选池，Spiderpool 根据 SpiderIPPool 对象中的优先级规则对这些候选者进行排序，然后按顺序从有空闲 IP 的 IP 池中开始选择 IP 地址进行分配。

## 获取候选池

Spiderpool 提供多种池选择规则，在为 Pod 分配 IP 地址时，会严格遵守 **高优先级到低优先级** 的池选择规则。以下规则按照从 **高优先级到低优先级** 的顺序列出，如果同时存在下面的多个规则，前一个规则将 **覆盖** 后一个规则。

- 优先级 1 ：SpiderSubnet 注解。

    SpiderSubnet 资源代表 IP 地址的集合，当需要为应用分配固定的 IP 地址时，应用管理员需要平台管理员告知可用的 IP 地址和路由属性等，但双方分属两个不同的运营部门，这使得每一个应用创建的工作流程繁琐，借助于 Spiderpool 的 SpiderSubnet 功能，它能自动从中子网分配 IP 给 IPPool，并且还能为应用固定 IP 地址，极大的减少了运维的成本。创建应用时可以使用 `ipam.spidernet.io/subnets` 或 `ipam.spidernet.io/subnet` 注解指定 Subnet，从而实现从子网中随机选取 IP 地址自动创建 IP 池，并从池中分配固定 IP 地址给应用。有关详情，请参阅 [SpiderSubnet](../usage/spider-subnet-zh_CN.md)。

- 优先级 2 ：SpiderIPPool 注解。

    一个 Subnet 中的不同 IP 地址，可分别存储到不同的 IPPool 实例中（Spiderpool 会校验 IPPool 之间的地址集合不重叠）。依据需求，SpiderIPPool 中的 IP 集合可大可小，能很好的应对 Underlay 网络的 IP 地址资源有限情况。同时，在创建应用时，结合 SpiderIPPool 注解 `ipam.spidernet.io/ippools` 或 `ipam.spidernet.io/ippool` 能绑定不同的 IPPool，也能分享相同的 IPPool，即既能够让所有应用共享使用同一个 Subnet，又能够实现 "微隔离"。有关详情，请参阅 [SpiderIPPool 注解](../reference/annotation.md)。

- 优先级 3 ：命名空间默认 IP 池。

    通过在命名空间中设置注解 `ipam.spidernet.io/default-ipv4-ippool` 或 `ipam.spidernet.io/default-ipv6-ippool` 指定默认的 IP 池。在该租户中创建应用时，如果没有其他高优先级的池规则，那么将从该租户可用的候选池中尝试分配 IP 地址。有关详情，请参阅 [命名空间注解](../reference/annotation.md)。

- 优先级 4 ：CNI 配置文件。

    通过在 CNI 配置文件中的 `default_ipv4_ippool` 和 `default_ipv6_ippool` 字段设置全局的 CNI 默认池，其可以设置多个 IP 池用作备选池，当应用使用该 CNI 配置网络时并调用 Spiderpool ，对于每个应用副本，Spiderpool 都会按照 "IP 池数组" 中元素的顺序依次尝试分配 IP 地址，在每个节点分属不同的地区或数据中心的场景，如果应用副本被调度到的节点，符合第一个 IP 池的节点亲和规则，Pod 会从该池中获得 IP 分配，如果不满足，Spiderpool 会尝试从备选池中选择 IP 池继续为 Pod 分配 IP ，直到所有备选池全部筛选失败。详细信息请参考 [CNI 配置](../reference/plugin-ipam.md)。

- 优先级 5 ：集群默认 IP 池。

    在 SpiderIPPool CR 对象中，可以通过将 **spec.default** 字段设置为 `true`，将池设置为集群默认 IPPool，默认为 `false`。详细信息请参考[集群默认 IPPool](../reference/crd-spiderippool.md)

## 过滤候选池

通过上述的池选择规则，获得 IPv4 和 IPv6 的候选 IP 池后，Spiderpool 会根据以下规则进行过滤，了解哪个候选 IP 池可用。

- IP 池处于候选者身份，但其处于 `terminating` 状态的，Spiderpool 将会过滤该池。

- IP 池的 `spec.disable` 字段用于设置 IP 池 是否可用，当该值为 `false` 时，意味着 IP 池不可使用。

- 检查 `IPPool.Spec.NodeName` 和 `IPPool.Spec.NodeAffinity` 属性是否与 Pod 的调度节点匹配。如果不匹配，则该 IP 池将被过滤。

- 检查 `IPPool.Spec.NamespaceName` 和 `IPPool.Spec.NamespaceAffinity` 属性是否与 Pod 的命名空间匹配。如果不匹配，则该 IP 池将被过滤。

- 检查 `IPPool.Spec.PodAffinity` 属性是否与 Pod 的 `matchLabels` 所匹配。如果不匹配，则该 IP 池将被过滤。

- 检查 `IPPool.Spec.MultusName` 属性是否与 Pod 当前 NIC Multus 配置匹配。如果不匹配，则该 IP 池将被过滤。

- 检查 IP 池的所有 IP 是不是都被 IPPool 实例的 `exclude_ips` 字段所包含，如果是，则该 IP 池将被过滤。

- 检查 IP 池的所有 IP 是不是都被 ReservedIP 实例所保留了，如果是，则该 IP 池将被过滤。

- IP 池的可用 IP 资源被耗尽，则该 IP 池也将被过滤。

## 候选池排序

过滤候选池后，可能仍存在多个候选池，Spiderpool 会进一步使用自定义优先级规则对这些候选者进行排序，然后按顺序从有空闲 IP 的 IP 池中开始选择 IP 地址进行分配。

- 具有 `IPPool.Spec.PodAffinity` 属性的 IP 池资源具有最高优先级。

- 具有 `IPPool.Spec.NodeName` 或 `IPPool.Spec.NodeAffinity` 属性的 IP 池资源具有第二高优先级（`NodeName` 的优先级高于 `NodeAffinity`）。

- 具有 `IPPool.Spec.NamespaceName` 或 `IPPool.Spec.NamespaceAffinity` 属性的 IP 池资源具有第三高优先级（`NamespaceName` 的优先级高于 `NamespaceAffinity`）。

- 具有 `IPPool.Spec.MultusName` 属性的 IP 池资源具有最低优先级。

> 这里有一些简单的例子来描述这个规则。
>
> 1. 具有属性 `IPPool.Spec.PodAffinity` 和 `IPPool.Spec.NodeName` 的 _IPPoolA_ 的优先级高于具有单一关联属性 `IPPool.Spec.PodAffinity` 的 _IPPoolB_。
> 2. 具有单个属性 `IPPool.Spec.PodAffinity` 的 _IPPoolA_ 的优先级高于具有属性 `IPPool.Spec.NodeName` 和 `IPPool.Spec.NamespaceName` 的 _IPPoolB_。
> 3. 具有属性 `IPPool.Spec.PodAffinity` 和 `IPPool.Spec.NodeName` 的 _IPPoolA_ 的优先级高于具有属性 `IPPool.Spec.PodAffinity`、`IPPool.Spec.NamespaceName` 和 `IPPool.Spec.MultusName` 的 _IPPoolB_ 。
>
> 如果 Pod 属于 StatefulSet，则会优先分配符合上面规则的 IP 地址。 一旦 Pod 重新启动，它将尝试重用最后分配的 IP 地址。
