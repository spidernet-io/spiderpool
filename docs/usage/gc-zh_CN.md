# 回收 IP

[**English**](./gc.md) | **简体中文**

Spiderpool 有一个 IP 垃圾回收机制，它可以帮助清理 CNI `cmdDel` 失败后泄漏的 IP。

## 启用 IP 回收支持

检查 `spiderpool-controller` Kubernetes 部署的 `SPIDERPOOL_GC_IP_ENABLED` 环境属性是否已设置为 `true`（默认已启用）。

```shell
kubectl edit deploy spiderpool-controller -n kube-system
```

## 设计

spiderpool-controller 使用 `Pod informer` 和定期间隔扫描所有 SpiderIPPool 实例来清理泄漏的 IP 及其相应的
SpiderEndpoint 对象。使用内存缓存来追踪应该清理的具有相应 IP 和 SpiderEndpoint 对象的 Pod。

以下是释放 IP 的几种情况：

* Pod 被 `deleted`，不包括 StatefulSet 重启其 Pod 的情况。

* Pod 正在 `Terminating`，我们将在 `pod.DeletionGracePeriodSeconds` 后释放其 IP，
  您可以设置 `AdditionalGraceDelay`（默认为 0 秒）环境变量以添加延迟时间。
  您还可以使用环境变量 `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`（默认已启用）确定是否回收
  `Terminating` 状态的 Pod。此环境变量可能在以下两种情况中有所帮助：
  
    1. 如果集群中的某个节点挂了，您必须依靠 IP GC 来释放这些 IP。
    2. 在某些基础模式下，如果不释放正在终止 Pod 的 IP，新 Pod 将因为缺少 IP 资源无法获取可用的 IP 去运行。

    然而，有一种特殊情况需要注意：如果节点由于接口或网络问题与 Master API 服务器失去连接，
    则 Pod 网络仍然可以正常工作。在这种情况下，如果我们释放其 IP 并将其分配给其他 Pod，会导致 IP 冲突。

* Pod 处于 `Succeeded` 或 `Failed` 阶段，我们将在 `pod.DeletionGracePeriodSeconds` 后清理 Pod 的 IP，
  您可以设置 `AdditionalGraceDelay`（默认为 0 秒）环境变量以添加延迟时间。

* SpiderIPPool 分配的 IP 所对应的 Pod 在 Kubernetes 中不存在，不包括 StatefulSet 重启其 Pod 的情况。

* Pod UID 不同于 SpiderIPPool IP 分配的 Pod UID。

## 注意事项

* `spiderpool-controller` 有多个副本并使用领导者选举。IP 垃圾回收 `pod informer` 仅为 `Master` 服务。
  但是，每个备份都会使用 `scan all SpiderIPPool` 以立即释放应清理的泄漏 IP。
  在上述 Pod 状态下，它不会追踪内存缓存中的 Pod。

* 我们可以使用环境变量 `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY`（默认为 5 秒）更改追踪 Pod `AdditionalGraceDelay`。

* 如果集群中有一个节点损坏，且启用了 `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED` 环境变量，
  IP GC 将强制释放不可达的 Pod 对应的 IP。还有一种罕见情况，即在 Pod 的 `DeletionGracePeriod` 时间之后，
  您的 Pod 仍然存活。IP GC 仍将强制释放无法访问的 Pod 对应的 IP。对于这两种情况，我们建议 Main CNI 具有检查
  IP 冲突的功能。[Veth](https://github.com/spidernet-io/plugins) 插件已经实现了此功能，
  您可以协调使用 `Macvlan` 或 `SR-IOV` CNI。
