#

![Spiderpool](./docs/images/spiderpool.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/spidernet-io/spiderpool)](https://goreportcard.com/report/github.com/spidernet-io/spiderpool)
[![CodeFactor](https://www.codefactor.io/repository/github/spidernet-io/spiderpool/badge)](https://www.codefactor.io/repository/github/spidernet-io/spiderpool)
[![codecov](https://codecov.io/gh/spidernet-io/spiderpool/branch/main/graph/badge.svg?token=YKXY2E4Q8G)](https://codecov.io/gh/spidernet-io/spiderpool)
[![Auto Version Release](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-version-release.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-version-release.yaml)
[![Auto Nightly CI](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/6009/badge)](https://bestpractices.coreinfrastructure.org/projects/6009)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/7e54bfe38fec206e7710c74ad55a5139/raw/spiderpoolcodeline.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/e1d3c092d1b9f61f1c8e36f09d2809cb/raw/spiderpoole2e.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/cd9ef69f5ba8724cb4ff896dca953ef4/raw/spiderpooltodo.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/38d00a872e830eedb46870c886549561/raw/spiderpoolperformance.json)

[**English**](./README.md) | **简体中文**

## Spiderpool 介绍

> Spiderpool 目前是一个 [CNCF Landscape](https://landscape.cncf.io/card-mode?category=cloud-native-network&grouping=category)级别的项目

Spiderpool 是一个 kubernetes 的 underlay 和 RDMA 网络解决方案，它增强了 [Macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
[SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) 的功能，它满足了包括但不仅限于以下的网络需求:

- Pod 按需接入到不同的 Underlay 网络
- Overlay 和 Underlay 需要共存于一个 Kubernetes 集群中
- Underlay CNIs 能够访问 Service 以及 Pod 健康检测问题
- 跨数据中心网络隔离时，多集群网络无法联通问题
- 用户不同的运行环境(裸金属，虚拟机或者公有云等)，需要一个统一的 Underlay CNI 解决方案
- 对于延迟敏感的应用，用户迫切需要降低网络延时

Spiderpool 使得 underlay 网络方案可应用在**裸金属、虚拟机和公有云环境**中，可为网络 I/O 密集性、低延时应用带来优秀的网络性能，包括**存储、中间件、AI 等应用**。详细的文档可参考[文档站](https://spidernet-io.github.io/spiderpool/)

## Spiderpool 功能描述

<div style="text-align:center">
  <img src="./docs/images/arch.png" alt="Your Image Description">
</div>

- 简化安装和使用

    当前开源社区对于 underlay CNI 的使用，需要手动安装 [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), [CNI-Plugins](https://github.com/containernetworking/plugins) 等诸多组件，Spiderpool 简化了安装流程，对相关的 CRD 进行了封装，提供了各种场景的完备文档，使得使用、管理更加便捷。

- 基于 CRD 的双栈 IPAM 能力

    提供了独享、共享的 IP 地址池，支持设置各种亲和性，为中间件等有状态应用和 kubevirt 等固定 IP 地址值，为无状态应用固定 IP 地址范围，自动化管理独享的 IP 池，优秀的 IP 回收避免 IP 泄露等。并且，具备优秀的 [IPAM 分配性能](./docs/concepts/ipam-performance-zh_CN.md) 。

- 使 Overlay 和 Underlay 网络能够共存于一个 Kubernetes 集群中

    可以通过为 Pod 插入多个 underlay CNI 网卡 或为 Pod 插入一个 overlay CNI 和 多个 underlay CNI 网卡，Pod 具备多种 CNI 网卡。Spiderpool 能够为多个
    underlay CNI 网卡定制不同的 IP 地址，调协所有网卡之间的策略路由，以确保请求向和回复向数据路径一致而避免丢包，从而使 Overlay 网络和多个 Underlay 网络共存于一个 Kubernetes 集群中。并且它使 [cilium](https://github.com/cilium/cilium), [calico](https://github.com/projectcalico/calico), [kubevirt](https://github.com/kubevirt/kubevirt) 等项目得到增强。

- 增强各种网络连通性

    打通 Pod 和宿主机的连通性，确保 Pod 健康检测工作正常，并可通过 kube-proxy 或 eBPF kube-proxy replacement 使得 Pod 访问 service，支持 Pod 的 IP 冲突检测、网关可达性检测等。多集群网络可基于相同的 underlay 网络或者 [Submariner](https://github.com/submariner-io/submariner) 实现联通。

- eBPF 增强

    kube-proxy replacement 技术极大加速了访问 service 场景，同节点上的 socket 短路技术加速了本地 Pod 的通信效率。相比 kube proxy 解析方式，[网络延时有最大 25% 的改善，网络吞吐有 50% 的提高]((./docs/concepts/io-performance-zh_CN.md))。

- RDMA

    提供了基于 RoCE、infiniband 技术下的 RDMA 解决方案。

- 网络双栈支持

    Spiderpool 组件和其提供的所有功能，支持 ipv4-only、ipv6-only、dual-stack 场景。

- 优秀的网络延时和吞吐量性能

    Spiderpool 在网络延时和吞吐量方面表现出色，超过了 overlay CNI，可参考 [性能报告](./docs/concepts/io-performance-zh_CN.md)

- 指标

## 为什么 Spiderpool 选择 macvlan、ipvlan、SR-IOV 为 datapath ？

- macvlan、ipvlan、SR-IOV 是承载 RDMA 网络加速的重要技术，RDMA 能为 AI 应用、延时敏感型应用、网络 I/O 密集型应用带来极大的性能提升，其网络性能大幅超过 overlay 网络解决方案。

- 区别于基于 veth 虚拟网卡的 CNI 解决方案，underlay 网络数据包避免了宿主机的三层网络转发，没有隧道封装开销，因此，它们能为应用提供了优秀的网络性能，包括优秀的网络吞吐量、低延时，节省了 CPU 的网络转发开销。

- 可直接对接 underlay 二层 VLAN 网络，应用可进行二层、三层网络通信，可进行组播、多播通信，数据包可受防火墙管控。

- 数据包携带 Pod 的真正 IP 地址，应用可直接基于 Pod IP 进行南北向通信，多云网络天然联通。

- underlay CNI 可基于宿主机不同的父网卡来创建虚拟机接口，因此可为存储、观测性等网络开销大的应用提供隔离的子网。

## Spiderpool 架构

Spiderpool 拥有清晰的架构设计，包括了如下的组件:

- _Spiderpool-controller_: 一组 Deployment，与 API-Server 交互, 管理多个 CRD 资源: 如 [SpiderIPPool](../reference/crd-spiderippool.md)、[SpiderSubnet](../reference/crd-spidersubnet.md)、[SpiderMultusConfig](../reference/crd-spidermultusconfig.md) 等, 实施这些 CRD 的校验、创建、状态。 并且响应来自 Spiderpool-agent Pod 的请求，分配、释放、回收、自动IP 池等功能。

- _Spiderpool-agent_: 一组 Daemonset，运行在每个节点。帮助安装 Multus、Coordinator、IPAM、CNI 等二进制文件到每个节点。响应 CNI 创建 Pod 时分配 IP 的请求，并与 Spiderpool-controller 交互，完成 Pod IP 的分配与释放。同时与 Coordinator 交互, 帮助 coordinator plugin 实施配置同步。

- _CNI Plugins_: 包括 Multus、Macvlan、IPVlan、Sriov-CNI、Rdma-CNI、Coordiantor、Ifacer 等。

- _[sriov-network operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator)_

- _[RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)_

更多细节参考 [架构](./docs/concepts/arch-zh_CN.md)。

## RoadMap

| 功能                               | macvlan  | ipvlan | SR-IOV    |
|----------------------------------|----------|---|-----------|
| Service By Kubeproxy             | Beta     |  Beta | Beta      |
| Service By Kubeproxy Replacement | Alpha    |  Alpha | Alpha     |
| Network Policy                   | In-plan  |  Alpha | In-plan   |
| Bandwidth                        | In-plan  | Alpha  | In-plan    |
| RDMA                             | Alpha    | Alpha | Alpha     |
| IPAM                             | Beta     | Beta | Beta      |
| Multi-Cluster                    | Alpha    | Alpha | Alpha     |
| Egress Policy                    | Alpha    | Alpha | Alpha     |
| 多网卡和路由调谐                         | beta     | beta | beta      |
| 适用场景                             | 裸金属      | 裸金属和虚拟机 | 裸金属       |

关于所有的功能规划，具体可参考 [roadmap](./docs/develop/roadmap.md)

## 快速开始

可参考 [快速搭建](./docs/usage/install/get-started-kind-zh_CN.md) 来使用 Spiderpool

参考 [使用](./docs/usage/readme.md) 来了解各种功能的使用方法

## Blogs

可参考 [Blogs](./docs/concepts/blog-zh_CN.md)

## Governance

[Maintainers and Committers](./docs/USERS.md)， 遵循 [governance document](./docs/develop/CODE-OF-CONDUCT.md).

## 使用者

使用了 Spiderpool 项目的 [用户](./docs/USERS.md).

## 参与开发

可参考 [开发搭建文档](./docs/develop/contributing.md).

## 联系我们

如果有任何关于 Spiderpool 的问题，欢迎您随时通过以下的方式联系我们👏:

- Slack: 如果你想在 CNCF Slack 加入 Spiderpool 的频道, 请先得到 CNCF Slack 的 **[邀请](https://slack.cncf.io/)**
  然后加入 [#Spiderpool](https://cloud-native.slack.com/messages/spiderpool) 的频道。

- 邮件: 您可以查看 [MAINTAINERS.md](https://github.com/spidernet-io/spiderpool/blob/main/MAINTAINERS.md) 获取所有维护者的邮箱地址， 联系邮箱地址以报告任何问题。

- 社区会议: 欢迎加入到我们每个月1号举行的[社区会议](https://docs.google.com/document/d/1tpNzxRWOz9-jVd30xGS2n5X02uXQuvqJAdNZzwBLTmI/edit?usp=sharing)，可以在这里讨论任何有关 Spiderpool 的问题。

- 微信群: 您可以扫描微信二维码，加入到 Spiderpool 技术交流群与我们进一步交流。

![Wechat QR-Code](./docs/images/wechat.png)

## License

Spiderpool is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for the full license text.

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="300"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="350"/>
<br/><br/>
Spiderpool 丰富了 <a href="https://landscape.cncf.io/?selected=spiderpool">CNCF 云原生全景图</a>。
</p>
