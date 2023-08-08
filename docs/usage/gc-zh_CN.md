# 回收 IP

[**English**](./gc.md) | **简体中文**

## 介绍

在 Kubernetes 中，垃圾回收（Garbage Collection，简称GC）对于 IP 地址的回收非常重要。IP 地址的可用性关系到 Pod 是否能够启动成功。GC 机制可以自动回收这些不再使用的 IP 地址，避免资源浪费和 IP 地址的耗尽。本文将介绍 Spiderpool 优秀的 GC 能力。

## 项目功能

在 IPAM 中记录了分配给 Pod 使用的 IP 地址，但是这些 Pod 在 Kubernetes 集群中已经不复存在，这些 IP 可称为 `僵尸 IP` ，Spiderpool 可针对 `僵尸 IP` 进行回收，它的实现原理如下：

在集群中 `delete Pod` 时，但由于`网络异常`或 `cni 二进制 crash` 等问题，导致调用 `cni delete` 失败，从而导致 IP 地址无法被 cni 回收。

- 在 `cni delete 失败` 等故障场景，如果一个曾经分配了 IP 的 Pod 被销毁后，但在 IPAM 中还记录分配着IP 地址，形成了僵尸 IP 的现象。Spiderpool 针对这种问题，会基于周期和事件扫描机制，自动回收这些僵尸 IP 地址。

节点意外宕机后，集群中的 Pod 永久处于 `deleting` 状态，Pod 占用的 IP 地址无法被释放。

- 对处于 `Terminating` 状态的 Pod，Spiderpool 将在 Pod 的 `spec.terminationGracePeriodSecond` 后，自动释放其 IP 地址。该功能可通过环境变量 `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED` 来控制。该能力能够用以解决 `节点意外宕机` 的故障场景。
