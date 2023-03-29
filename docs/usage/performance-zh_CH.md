# Spiderpool 性能测试

*[Spiderpool](https://github.com/spidernet-io/spiderpool) 是一个适用于 underlay 网络的高性能 IPAM CNI 插件，此文将对比其与市面上主流的 underlay IPAM CNI 插件（如 [Whereabouts](https://github.com/k8snetworkplumbingwg/whereabouts)，[Kube-OVN](https://github.com/kubeovn/kube-ovn)）以及被广泛使用的 overlay IPAM CNI 插件 [calico-ipam](https://github.com/projectcalico/calico) 在 ”1000 Pod“ 场景下的性能表现。*

## 背景

为什么要做 underlay IPAM CNI 插件的性能测试？

1. IPAM 分配 IP 地址的速度，很大程度上的决定了应用发布的速度。
2. 大规模的 Kubernetes 集群在故障恢复时，underlay IPAM 往往会成为性能瓶颈。
3. underlay 网络下，私有的 IPv4 地址有限。在有限的 IP 地址范围内，并发的创建 Pod 会涉及 IP 地址的抢占与冲突，能否快速的调节好有限的 IP 地址资源具有挑战。

## 环境

- Kubernetes: `v1.25.4`
- container runtime: `containerd 1.6.12`
- OS: `CentOS Linux 8`
- kernel: `4.18.0-348.7.1.el8_5.x86_64`

| Node     | Role          | CPU  | Memory |
| -------- | ------------- | ---- | ------ |
| master1  | control-plane | 4C   | 8Gi    |
| master2  | control-plane | 4C   | 8Gi    |
| master3  | control-plane | 4C   | 8Gi    |
| worker4  |               | 3C   | 8Gi    |
| worker5  |               | 3C   | 8Gi    |
| worker6  |               | 3C   | 8Gi    |
| worker7  |               | 3C   | 8Gi    |
| worker8  |               | 3C   | 8Gi    |
| worker9  |               | 3C   | 8Gi    |
| worker10 |               | 3C   | 8Gi    |

## 测试对象

本次测试基于 `0.3.1` 版本的 [CNI Specification](https://www.cni.dev/docs/spec/)，以 [macvlan](https://www.cni.dev/plugins/current/main/macvlan/) 搭配 Spiderpool 作为测试方案，并选择了开源社区中其它几种对接 underlay 网络的方案作为对比：

| Main CNI            | Main CNI 版本 | IPAM CNI                  | IPAM CNI 版本 | 特点                                                         |
| ------------------- | ------------- | ------------------------- | ------------- | ------------------------------------------------------------ |
| macvlan             | `v1.1.1`      | Spiderpool                | `v0.4.0`      | 集群中存在多个 IP 池，每个池中的 IP 地址都可以被集群中的任意一个节点上的 Pod 所使用，当集群中的多个 Pod 并发的从同一个池中分配 IP 地址时，存在竞争。支持托管 IP 池的全生命流程，使其同步的与工作负载创建、扩缩容、删除，弱化了过大的共享池所带来的并发或存储问题。 |
| macvlan             | `v1.1.1`      | Whereabouts (CRD backend) | `v0.6.1`      | 各节点可以定义各自可用的 IP 池范围，若节点间存在重复定义的 IP 地址，那么这些 IP 地址上升为一种共享资源。 |
| Kube-OVN (underlay) | `v1.11.3`     | Kube-OVN                  | `v1.11.3`     | 以子网来组织 IP 地址，每个 Namespace 可以归属于特定的子网， Namespace 下的 Pod 会自动从所属的子网中获取 IP 地址。子网也是一种集群资源，同一个子网的 IP 地址可以分布在任意一个节点上。 |
| Calico              | `v3.23.3`     | calico-ipam (CRD backend) | `v3.23.3`     | 每个节点独享一个或多个 IP block，各节点上的 Pod 仅使用本地 IP block 中的 IP 地址，节点间无任何竞争与冲突，分配的效率非常高。 |

## 方案

测试期间，我们会遵循如下约定：

- IPv4/IPv6 双栈场景。
- 测试 underlay IPAM CNI 插件时，尽最大可能的确保可用的 IP 地址数量与 Pod 数量为 **1:1**。例如，接下来我们计划创建 1000 个 Pod，那么应当限制可用的 IPv4/IPv6 地址数量均为 1000 个。

具体的，我们会尝试以如下两种方式在上述 Kubenetes 集群上来启动总计 1000 个 Pod，并记录所有 Pod 均达到 `Running` 的耗时：

- 仅创建一个 Deployment，其副本数为 1000。
- 创建 100 个 Deployment，每个 Deployment 的副本数为 10。

接下来，我们会使用如下命令一次性的删除这 1000 个 Pod，记录被重建的 1000 个 Pod 再次全部达到 `Running` 的耗时：

```bash
kubectl get pod | grep "prefix" | awk '{print $1}' | xargs kubectl delete pod
```

最后，我们删除所有的 Deployment，记录所有 Pod 完全消失的耗时。

## 结果

### 单个 1000 副本的 Deployment

| CNI                   | 创建   | 重建  | 删除  |
| --------------------- | ------ | ----- | ----- |
| macvlan + Spiderpool  | 2m35s  | 9m50s | 1m50s |
| macvlan + Whereabouts | 25m18s | 失败  | 3m5s  |
| Kube-OVN              | 3m55s  | 7m20s | 2m13s |
| Calico + calico-ipam  | 1m56s  | 4m6s  | 1m36s |

> 在测试 macvlan + Whereabouts 这个组合期间，创建的场景中 922 个 Pod 在 14m25s 内以较为均匀的速率达到 `Running` 状态，自此之后的 Pod 增长速率大幅降低，最终 1000 个 Pod 花了 25m18s 达到 `Running` 状态。至于重建的场景，在 55 个 Pod 达到 `Running` 状态后，Whereabouts 就基本不工作了，耗时类比于正无穷。

### 100 个 10 副本的 Deployment

| CNI                   | 创建   | 重建  | 删除  |
| --------------------- | ------ | ----- | ----- |
| macvlan + Spiderpool  | 1m37s  | 3m27s | 1m22s |
| macvlan + Whereabouts | 21m49s | 失败  | 2m9s  |
| Kube-OVN              | 4m6s   | 7m46s | 2m8s  |
| Calico + calico-ipam  | 1m57s  | 3m58s | 1m35s |

## 小结

虽然 Spiderpool 是一种适用于 underlay 网络的 IPAM CNI 插件，其相较于主流的 overlay IPAM CNI 插件，面临着更多的复杂的 IP 地址抢占与冲突的问题，但它在大多数场景下的性能表现亦不逊色于后者。
