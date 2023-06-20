# Spdiderpool：如何解决僵尸 IP 的回收

## Underlay 网络解决方案

为什么需要 Underlay 网络解决方案？在数据中心私有云中，有许多需要 Underlay 网络的应用场景：

* 低延迟和高吞吐量：在一些需要低延迟和高吞吐量的应用场景中，Underlay 网络方案通常比 Overlay 网络方案更具优势。由于 Underlay 网络是基于物理网络构建的，因此可以提供更快速和稳定的网络传输服务。

* 传统主机应用上云：在数据中心内，许多传统主机应用仍然使用传统的网络对接方式，例如服务暴露和发现、多子网对接等。在这种情况下，使用 Underlay 网络解决方案可以更好地满足这些应用的需求。

* 数据中心网络管理：数据中心管理人员通常需要对应用实施安全管控，例如使用防火墙、Vlan 隔离等手段。此外，他们还需要使用传统的网络观测手段实施集群网络监控。使用 Underlay 网络解决方案可以更方便地实现这些需求。

* 独立的宿主机网卡规划：在一些特殊的应用场景下，例如 Kubevirt、存储项目、日志项目等，需要实施独立的宿主机网卡规划，以保证底层子网的带宽隔离。使用Underlay 网络解决方案可以更好地支持这些应用的需求，从而提高应用的性能和可靠性。

随着数据中心私有云的不断普及，Underlay 网络作为数据中心网络架构的重要组成部分，已经被广泛应用于数据中心的网络架构中，以提供更高效的网络传输和更好的网络拓扑管理能力。

## Underlay 网络中的僵尸 IP 问题

什么是僵尸 IP ？

* 在 IPAM 中记录了分配给 Pod 使用的 IP 地址，但是这些 Pod 在 Kubernetes 集群中已经不复存在，这些 IP 可称为 僵尸 IP 。

在实际的生产中，难免会遇到集群中出现僵尸 IP ，如：

* 在集群中 `delete Pod` 时，但由于`网络异常`或 `cni 二进制 crash` 等问题，导致调用 `cni delete` 失败，从而导致 IP 地址无法被 cni 回收。

* 节点意外宕机后，集群中的 Pod 永久处于 `deleting` 状态，Pod 占用的 IP 地址无法被释放。

在使用 Underlay 网络的 Kubernetes 集群中，当出现僵尸 IP 时，可能会带来如下的一些问题：

* Underlay 网络下 IP 资源有限：在大规模集群中，Pod 的数量可能非常庞大，IPAM 会为每个 Pod 实例分配指定的 Underlay 子网 IP 以进行网络通信，如果出现僵尸 IP 问题，可能会导致大量的 IP 资源浪费，或将面临无 Underlay IP 资源可用的局面。

* 固定 IP 需求，导致新 Pod 启动失败：将一个拥有 10 个 IP 地址的 IP 池，固定给 10 个副本的应用使用，如出现上述的僵尸 IP 问题，旧的 Pod IP 无法被回收，新 Pod 将因为缺少 IP 资源，且无法获取到可用的 IP，从而无法启动。这将对应用的稳定性和可靠性造成威胁，甚至可能导致整个应用程序都无法正常运行。

## 解决方案：Spiderpool

[Spiderpool](https://github.com/spidernet-io/spiderpool) 是一个 Kubernetes 的 Underlay 网络解决方案，通过提供轻量级的 `meta` 插件和 `IPAM` 插件，Spiderpool 灵活地整合与强化了开源社区中现有的 CNI 项目，最大程度的简化 Underlay 网络下 IPAM 的运维工作，使得多 CNI 协同工作真正的可落地，支持运行在裸金属、虚拟机、公有云等环境下。

Spiderpool 通过如下的 IP 回收机制，以解决 Underlay 网络中出现的故障 IP 问题：

* 对处于 `Terminating` 状态的 Pod，Spiderpool 将在 Pod 的 `spec.terminationGracePeriodSecond` 后，自动释放其 IP 地址。该功能可通过环境变量 `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED` 来控制。该能力能够用以解决 `节点意外宕机` 的故障场景。
  
* 在 `cni delete 失败` 等故障场景，如果一个曾经分配了 IP 的 Pod 被销毁后，但在 IPAM 中还记录分配着IP 地址，形成了僵尸 IP 的现象。Spiderpool 针对这种问题，会基于周期和事件扫描机制，自动回收这些僵尸 IP 地址。

## 等比例 IP 分配测试

IPAM 要求能够精准的分配 IP 地址，同时 Spiderpool 还具备了健壮的故障 IP 回收能力，笔者做了如下的等比例 IP 分配测试，来进行验证。本次 `等比例 IP 分配测试` 基于 0.3.1 版本的 CNI Specification，以 Macvlan 搭配 Spiderpool（版本 v0.6.0）作为测试方案，并选择了开源社区中的 Whereabouts（版本 v0.6.2）搭配 Macvlan 、Kube-ovn（版本 v1.11.8）、Calico-ipam（版本 v3.26.1）几种网络解决方案为对比，测试场景如下：

1. 创建 1000 个 Pod，同时限制可用的 IPv4/IPv6 地址数量均为 1000 个，确保可用的 IP 地址数量与 Pod 数量为 1:1。

2. 使用如下命令一次性的重建这 1000 个 Pod，记录被重建的 1000 个 Pod 全部 `Running` 的耗时。验证在固定 IP 地址时，并发重建的 Pod 在涉及 IP 地址的回收、抢占与冲突的场景下，各 IPAM 插件能否快速的调节好有限的 IP 地址资源，保证应用恢复的速度。

    ```bash
    ~# kubectl get pod | grep "prefix" | awk '{print $1}' | xargs kubectl delete pod
    ```

3. 将所有节点下电后再上电，模拟故障恢复，记录 1000 个 Pod 再次达到 `Running` 的耗时。

4. 删除所有的 Deployment，记录所有 Pod 完全消失的耗时。

测试数据如下：

  | type                   | 创建   | 重建   | 故障恢复 | 删除  |
  | ---------------------- | ------ | ----- | -------- | ----- |
  |  Spiderpool（v0.6.0）  | 1m37s  | 3m27s | 3m3s     | 1m22s |
  |  Whereabouts（v0.6.2） | 21m49s | 失败  | 失败     | 2m9s  |
  |  Kube-OVN（v1.11.8）   | 4m6s   | 7m46s | 10m22s   | 2m8s  |
  |  Calico（v3.26.1）     | 1m57s  | 3m58s | 4m16s    | 1m35s |

* Spiderpool，Kube-ovn 的 IPAM 分配原理，是整个集群节点的所有 Pod 都从同一个 CIDR 中分配 IP，所以 IP 分配和释放需要面临激烈的竞争，IP 分配性能的挑战会更大；Whereabouts 和 Calico 的 IPAM 分配原理，是每个节点都有一个小的 IP 集合，所以 IP 分配的竞争比较小，IP 分配性能的挑战会小。但从实验数据上看，虽然 Spdierpool 的 IPAM 原理是 "吃亏" 的，但是分配 IP 的性能却是很好的。

* 在测试 Macvlan + Whereabouts 这个组合期间，创建的场景中 922 个 Pod 在 14m25s 内以较为均匀的速率达到 `Running` 状态，自此之后的 Pod 增长速率大幅降低，最终 1000 个 Pod 花了 25m18s 达到 `Running` 状态。至于重建的场景，在 55 个 Pod 达到 `Running` 状态后，Whereabouts 无法继续分配 IP 给 Pod。由于测试场景中 IP 地址数量与 Pod 数量为 1:1，如果 IPAM 组件未能正确的回收 IP，新 Pod 将因为缺少 IP 资源，且无法获取到可用的 IP，从而无法启动。

## 结论

Spiderpool 在各种测试场景下表现优秀。虽然 Spiderpool 是一种适用于 Underlay 网络下的 IPAM 解决方案，其 IP 分配以及回收的特点相较于主流的 Overlay IPAM CNI 插件，面临着更多的、复杂的 IP 地址抢占与冲突的问题，但它的表现仍不逊色于后者。
