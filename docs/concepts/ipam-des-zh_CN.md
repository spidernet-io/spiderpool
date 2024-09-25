# IPAM

**简体中文** | [**English**](./ipam-des.md)

## Underlay 网络和 Overlay 网络的 IPAM

云原生网络中出现了两种技术类别："Overlay 网络方案" 和 "Underlay 网络方案"。
云原生网络对于它们没有严格的定义，我们可以从很多 CNI 项目的实现原理中，简单抽象出这两种技术流派的特点，它们可以满足不同场景下的需求。

Spiderpool 是为 Underlay 网络特点而设计，以下对两种方案进行比较，能够更好说明 Spiderpool 的特点和使用场景。

### Overlay 网络方案 IPAM

本方案实现了 Pod 网络同宿主机网络的解耦，例如 [Calico](https://github.com/projectcalico/calico)、[Cilium](https://github.com/cilium/cilium) 等 CNI 插件，
这些插件多数使用了 vxlan 等隧道技术，搭建起一个 Overlay 网络平面，再借用 NAT 技术实现南北向的通信。

这类技术流派的 IPAM 分配特点是：

1. Pod 子网中的 IP 地址按照节点进行了分割

      以一个更小子网掩码长度为单位，把 Pod subnet 分割出更小的 IP block 集合，依据 IP 使用的用量情况，每个 node 都会获取到一个或者多个 IP block。

      这意味着两个特点：第一，每个 node 上的 IPAM 插件只需要在本地的 IP block 中分配和释放 IP 地址时，与其它 node 上的 IPAM 无 IP 分配冲突，IPAM 分配效率更高。
      第二，某个具体的 IP 地址跟随 IP block 集合，会相对固定的一直在某个 node 上被分配，没法随同 Pod 一起被调度漂移。

2. IP 地址资源充沛

      只要 Pod 子网不与相关网络重叠，再能够合理利用 NAT 技术，Kubernetes 单个集群可以拥有充沛的 IP 地址资源。
      因此，应用不会因为 IP 不够而启动失败，IPAM 组件面临的异常 IP 回收压力较小。

3. 没有应用 "IP 地址固定"需求

      对于应用 IP 地址固定需求，有无状态应用和有状态应用的区别：对于 Deployment 这类无状态应用，因为 Pod 名称会随着 Pod 重启而变化，
      应用本身的业务逻辑也是无状态的，因此对于 "IP 地址固定" 的需求，只能让所有 Pod 副本固定在一个 IP 地址的集合内；对于 StatefulSet
      这类有状态应用，因为 Pod name 等信息都是固定的，应用本身的业务逻辑也是有状态的，因此对于 "IP 地址固定"需求，要实现单个 Pod 和具体 IP 地址的强绑定。

      在 "Overlay 网络方案"方案下，多是借助了 NAT 技术向集群外部暴露服务的入口和源地址，借助 DNS、clusterIP 等技术来实现集群东西向通信。
      其次，IPAM 的 IP block 方式把 IP 相对固定到某个节点上，而不能保证应用副本的跟随调度。
      因此，应用的 "IP 地址固定"能力无用武之地，当前社区的主流 CNI 多数不支持 "IP 地址固定"，或者支持方法较为简陋。

这个方案的优点是，无论集群部署在什么样的底层网络环境上，CNI 插件的兼容性都非常好，且都能够为 Pod 提供子网独立、IP 地址资源充沛的网络。

### Underlay 网络方案 IPAM

本方案实现了 Pod 共享宿主机的底层网络，即 Pod 直接获取宿主机网络中的 IP 地址。这样，应用可直接使用自己的 IP 地址进行东西向和南北向通信。

Underlay 网络方案的实施，有两种典型的场景：一种是集群部署实施在"传统网络"上；一种是集群部署在 IAAS 环境上，例如公有云。以下总结了"传统网络场景"的 IPAM 特点：

1. 单个 IP 地址应该能够在任一节点上被分配

      这个需求有多方面的原因：随着数据中心的网络设备增加、多集群技术的发展，IPv4 地址资源稀缺，要求 IPAM 提高 IP 资源的使用效率；
      对于有 "IP 地址固定"需求的应用，其 Pod 副本可能会调度到集群的任意一个节点上，并且，在故障场景下还会发生节点间的漂移，要求 IP 地址一起漂移。

      因此，在集群中的任意一个节点上，一个 IP 地址应该具备能够被分配给 Pod 使用的可能。

2. 同一应用的不同副本，能实现跨子网获取 IP 地址

      例如，一个集群中，宿主机1的区域只能使用子网 172.20.1.0/24，而宿主机2的区域只能使用子网 172.20.2.0/24，在此背景下，
      当一个应用跨子网部署副本时，要求 IPAM 能够在不同的节点上，为同一个应用下的不同 Pod 分配出子网匹配的 IP 地址。

3. 应用 IP 地址固定

      很多传统应用在云化改造前，是部署在裸金属环境上的，服务之间的网络未引入 NAT 地址转换，微服务架构中需要感知对方的源 IP 或目的 IP，
      并且，网络管理员也习惯了使用防火墙等手段来精细管控网络安全。

      因此，应用上云后，无状态应用希望能够实现 IP 范围的固定，有状态应用希望能够实现 IP 地址的唯一对应，这样，能够减少对微服务架构的改造工作。

4. 一个 Pod 的多网卡获取不同子网的 IP 地址

      既然是对接 Underlay 网络，Pod 就会有多网卡需求，以使其通达不同的 Underlay 子网，这要求 IPAM 能够给应用的不同网卡分配不同子网下的 IP 地址。

5. IP 地址冲突

      在 Underlay 网络中，更加容易出现 IP 冲突，例如，Pod 与集群外部的主机 IP 发生了冲突，与其它对接了相同子网的集群冲突，
      而 IPAM 组件很难感知外部这些冲突的 IP 地址，多需要借助 CNI 插件进行实时的 IP 冲突检测。

6. 已用 IP 地址的释放回收

      因为 Underlay 网络 IP 地址资源的稀缺性，且应用有 IP 地址固定需求，所以，"应当"被释放的 IP 地址若未被 IPAM 组件回收，新启动的 Pod 可能会因为缺少 IP 地址而失败。
      这就要求 IPAM 组件拥有更加精准、高效、及时的 IP 回收机制。

这个方案的优势有：无需网络 NAT 映射的引入，对应用的云化网络改造，提出了最大的便利；底层网络的火墙等设备，可对 Pod 通信实现相对较为精细的管控；无需隧道技术，
网络通信的吞吐量和延时性能也相对的提高了。

## Spiderpool IPAM

任何支持第三方 IPAM 插件的 CNI 项目，都可以配合 Spiderpool IPAM 插件，例如：
[macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
[sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
[ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni),
[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni)
[calico CNI](https://github.com/projectcalico/calico),
[weave CNI](https://github.com/weaveworks/weave)

### Spiderpool IP 分配算法

当 Pod 创建时，它将按照以下步骤获取 IP 分配；IP 分配生命周期将经历 `获取候选池`、`过滤候选池`、`候选池排序` 三个大阶段。

- 获取候选池：Spiderpool 有多种池选择规则，会严格遵守 **高优先级到低优先级** 的池选择规则，获取**高优先级规则**命中的所有池，将它们标记为候选者身份，以有资格被进一步考虑。

- 过滤候选池：Spiderpool 通过亲和性等过滤机制，更精确地从所有候选池中选择合适的候选池，以满足特定的需求或复杂的使用场景。

- 候选池排序：对于多候选池，Spiderpool 根据 SpiderIPPool 对象中的优先级规则对这些候选者进行排序，然后按顺序从有空闲 IP 的 IP 池中开始选择 IP 地址进行分配。

#### 获取候选池

Spiderpool 提供多种池选择规则，在为 Pod 分配 IP 地址时，会严格遵守 **高优先级到低优先级** 的池选择规则。以下规则按照从 **高优先级到低优先级** 的顺序列出，如果同时存在下面的多个规则，前一个规则将 **覆盖** 后一个规则。

- 优先级 1 ：SpiderSubnet 注解。

    SpiderSubnet 资源代表 IP 地址的集合，当需要为应用分配固定的 IP 地址时，应用管理员需要平台管理员告知可用的 IP 地址和路由属性等，但双方分属两个不同的运营部门，这使得每一个应用创建的工作流程繁琐，借助于 Spiderpool 的 SpiderSubnet 功能，它能自动从中子网分配 IP 给 IPPool，并且还能为应用固定 IP 地址，极大的减少了运维的成本。创建应用时可以使用 `ipam.spidernet.io/subnets` 或 `ipam.spidernet.io/subnet` 注解指定 Subnet，从而实现从子网中随机选取 IP 地址自动创建 IP 池，并从池中分配固定 IP 地址给应用。有关详情，请参阅 [SpiderSubnet](../usage/spider-subnet-zh_CN.md)。

- 优先级 2 ：SpiderIPPool 注解。

    一个 Subnet 中的不同 IP 地址，可分别存储到不同的 IPPool 实例中（Spiderpool 会校验 IPPool 之间的地址集合不重叠）。依据需求，SpiderIPPool 中的 IP 集合可大可小。能很好的应对 Underlay 网络的 IP 地址资源有限情况，且这种设计特点，创建应用时，结合 SpiderIPPool 注解 `ipam.spidernet.io/ippools` 或 `ipam.spidernet.io/ippool` 能绑定不同的 IPPool，也能分享相同的 IPPool，既能够让所有应用共享使用同一个 Subnet，又能够实现 "微隔离"。有关详情，请参阅 [SpiderIPPool 注解](../reference/annotation.md)。

- 优先级 3 ：命名空间默认 IP 池。

    通过在命名空间中设置注解 `ipam.spidernet.io/default-ipv4-ippool` 或 `ipam.spidernet.io/default-ipv6-ippool` 指定默认的 IP 池。在该租户中创建应用时，如果没有其他高优先级的池规则，那么将从该租户可用的候选池中尝试分配 IP 地址。有关详情，请参阅 [命名空间注解](../reference/annotation.md)。

- 优先级 4 ：CNI 配置文件。

    通过在 CNI 配置文件中的 `default_ipv4_ippool` 和 `default_ipv6_ippool` 字段设置全局的 CNI 默认池，其可以设置多个 IP 池用作备选池，当应用使用该 CNI 配置网络时并调用 Spiderpool ，对于每个应用副本，Spiderpool 都会按照 "IP 池数组" 中元素的顺序依次尝试分配 IP 地址，在每个节点分属不同的地区或数据中心的场景，如果应用副本被调度到的节点，符合第一个 IP 池的节点亲和规则，Pod 会从该池中获得 IP 分配，如果不满足，Spiderpool 会尝试从备选池中选择 IP 池继续为 Pod 分配 IP ，直到所有备选池全部筛选失败。详细信息请参考[CNI 配置](../reference/plugin-ipam.md)。

- 优先级 5 ：集群默认 IPPool。

    在 SpiderIPPool CR 对象中，可以通过将 **spec.default** 字段设置为 `true`，将池设置为集群默认 IPPool，默认为 `false`。详细信息请参考[集群默认 IPPool](../reference/crd-spiderippool.md)

#### 过滤候选池

通过上述的池选择规则，获得 IPv4 和 IPv6 的 IPPool 候选后，Spiderpool 会根据以下规则进行过滤，了解哪个候选 IPPool 可用。

- IP 池处于候选者身份，但其处于 `terminating` 状态的，Spiderpool 将会过滤该池。

- IP 池的 `spec.disable` 字段用于设置 IPPool 是否可用，当该值为 `true` 时，意味着 IPPool 不可使用。

- 检查 `IPPool.Spec.NodeName` 和 `IPPool.Spec.NodeAffinity` 属性是否与 Pod 的调度节点匹配。 如果不匹配，则该 IPPool 将被过滤。

- 检查 `IPPool.Spec.NamespaceName` 和 `IPPool.Spec.NamespaceAffinity` 属性是否与 Pod 的命名空间匹配。如果不匹配，则该 IPPool 将被过滤。

- 检查 `IPPool.Spec.PodAffinity` 属性是否与 Pod 的 `matchLabels` 所匹配。如果不匹配，则该 IPPool 将被过滤。

- 检查 `IPPool.Spec.MultusName` 属性是否与 Pod 当前 NIC Multus 配置匹配。如果不匹配，则该 IPPool 将被过滤。

- 检查 IPPool 所有 IP 是不是都被 IPPool 实例的 `exclude_ips` 字段所包含，如果是，则该 IPPool 将被过滤。

- 检查 IPPool 所有 IP 是不是都被 ReservedIP 实例所保留了，如果是，则该 IPPool 将被过滤。

- IPPool 的可用 IP 资源被耗尽，则该 IPPool 也将被过滤。

#### 候选池排序

过滤候选池后，可能仍存在多个候选池，Spiderpool 会进一步使用自定义优先级规则对这些候选者进行排序，然后按顺序从有空闲 IP 的 IP 池中开始选择 IP 地址进行分配。

- 具有 `IPPool.Spec.PodAffinity` 属性的 IPPool 资源具有最高优先级。

- 具有 `IPPool.Spec.NodeName` 或 `IPPool.Spec.NodeAffinity` 属性的 IPPool 资源具有第二高优先级。（`NodeName` 的优先级高于 `NodeAffinity`）。

- 具有 `IPPool.Spec.NamespaceName` 或 `IPPool.Spec.NamespaceAffinity` 属性的 IPPool 资源具有第三高优先级。（`NamespaceName` 的优先级高于 `NamespaceAffinity`）。

- 具有 `IPPool.Spec.MultusName` 属性的 IPPool 资源具有最低优先级。

> 注意：这里有一些简单的例子来描述这个规则。
>
> 1. 具有属性 `IPPool.Spec.PodAffinity` 和 `IPPool.Spec.NodeName` 的 _IPPoolA_ 的优先级高于具有单一关联属性 `IPPool.Spec.PodAffinity` 的 _IPPoolB_。
> 2. 具有单个属性 `IPPool.Spec.PodAffinity` 的 _IPPoolA_ 的优先级高于具有属性 `IPPool.Spec.NodeName` 和 `IPPool.Spec.NamespaceName` 的 _IPPoolB_。
> 3. 具有属性 `IPPool.Spec.PodAffinity` 和 `IPPool.Spec.NodeName` 的 _IPPoolA_ 的优先级高于具有属性 `IPPool.Spec.PodAffinity`、`IPPool.Spec.NamespaceName` 和 `IPPool.Spec.MultusName` 的 _IPPoolB_ 。

NOTE：

> 如果 Pod 属于 StatefulSet，则会优先分配符合上面规则的 IP 地址。 一旦 Pod 重新启动，它将尝试重用最后分配的 IP 地址。

## IP 回收机制

在 Kubernetes 中，垃圾回收（Garbage Collection，简称GC）对于 IP 地址的回收非常重要。IP 地址的可用性关系到 Pod 是否能够启动成功。GC 机制可以自动回收这些不再使用的 IP 地址，避免资源浪费和 IP 地址的耗尽。

在 IPAM 中记录了分配给 Pod 使用的 IP 地址，但是这些 Pod 在 Kubernetes 集群中已经不复存在，这些 IP 可称为 `僵尸 IP` ，Spiderpool 可针对 `僵尸 IP` 进行回收，它的实现原理如下：

在集群中 `delete Pod` 时，但由于`网络异常`或 `cni 二进制 crash` 等问题，导致调用 `cni delete` 失败，从而导致 IP 地址无法被 cni 回收。

- 在 `cni delete 失败` 等故障场景，如果一个曾经分配了 IP 的 Pod 被销毁后，但在 IPAM 中还记录分配着IP 地址，形成了僵尸 IP 的现象。Spiderpool 针对这种问题，会基于周期和事件扫描机制，自动回收这些僵尸 IP 地址。
- 因其他意外导致 **无状态** Pod 一直处于 `Terminating` 阶段，Spiderpool 将在 Pod 的 `spec.terminationGracePeriodSecond` + [spiderpool-controller ENV](./../reference/spiderpool-controller.md#env) `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY` 时间后，自动释放其 IP 地址。该功能可通过环境变量 `SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_READY_NODE_ENABLED` 来控制。该能力能够用以解决 `节点正常但 Pod 删除失败` 的故障场景。

节点意外宕机后，集群中的 Pod 永久处于 `Terminating` 阶段，Pod 占用的 IP 地址无法被释放。

- 对处于 `Terminating` 阶段的 **无状态** Pod，Spiderpool 将在 Pod 的 `spec.terminationGracePeriodSecond` + [spiderpool-controller ENV](./../reference/spiderpool-controller.md#env) `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY` 时间后，自动释放其 IP 地址。该功能可通过环境变量 `SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED` 来控制。该能力能够用以解决 `节点意外宕机` 的故障场景。
