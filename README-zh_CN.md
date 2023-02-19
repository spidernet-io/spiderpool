# Spiderpool

[![Go Report Card](https://goreportcard.com/badge/github.com/spidernet-io/spiderpool)](https://goreportcard.com/report/github.com/spidernet-io/spiderpool)
[![CodeFactor](https://www.codefactor.io/repository/github/spidernet-io/spiderpool/badge)](https://www.codefactor.io/repository/github/spidernet-io/spiderpool)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=spidernet-io_spiderpool&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=spidernet-io_spiderpool)
[![codecov](https://codecov.io/gh/spidernet-io/spiderpool/branch/main/graph/badge.svg?token=YKXY2E4Q8G)](https://codecov.io/gh/spidernet-io/spiderpool)
[![Auto Version Release](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-version-release.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-version-release.yaml)
[![Auto Nightly CI](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/6009/badge)](https://bestpractices.coreinfrastructure.org/projects/6009)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/7e54bfe38fec206e7710c74ad55a5139/raw/spiderpoolcodeline.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/93b7ba26a4600fabe100ff640f9b3bd3/raw/spiderpoolcomment.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/e1d3c092d1b9f61f1c8e36f09d2809cb/raw/spiderpoole2e.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/cd9ef69f5ba8724cb4ff896dca953ef4/raw/spiderpooltodo.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/38d00a872e830eedb46870c886549561/raw/spiderpoolperformance.json)

[**English**](./README.md) | **简体中文**


## Spiderpool 介绍

Spiderpool 是一个 kubernetes 的 IPAM 插件项目， 其针对 underlay 网络的 IP 地址管理需求而设计，能够为任何兼容第三方 IPAM 插件的 CNI 项目所使用。

为什么希望研发 Spiderpool ? 面对 underlay 网络的很多 IPAM 需求，当前开源社区中并未提供全面、友好、智能的开源解决方案，因此，Spiderpool 克服了 underlay 网络分配 IP 地址的复杂性，
希望 IP 分配的运维工作像一些 overlay 网络模型一样简单，它包括应用 IP 地址固定、IP 地址自动弹性扩缩容、 多网卡支持、双栈支持等特点，
本项目希望能够给开源爱好者一个新的 IPAM 选择。

## underlay 网络 和 overlay 网络场景的 IPAM 

云原生网络中出现了两种技术类别，" overlay 网络方案" 和 " underlay网络方案"，
云原生网络对于它们没有严格的定义，我们可以从很多 CNI 项目的实现原理中，简单抽象出这两种技术流派的特点，它们可以满足不同场景下的需求。
 Spiderpool 是为 underlay 网络特点而设计，以下对两种方案进行比较，能够更好说明 Spiderpool 的特点和使用场景。

### overlay 网络方案

本方案实现了 POD 网络同宿主机网络的解耦，例如 [Calico](https://github.com/projectcalico/calico) , [Cilium](https://github.com/cilium/cilium) 等 CNI 插件，
它们或者使用 vxlan 等隧道技术搭建起一个 overlay 网络平面，再借用 NAT 技术实现南北向的通信，它们或者借用 BGP 路由协议打通宿主机网络的路由，使得宿主机的网络能够转发 POD 的
东西向和南北向流量。

这类技术流派的 IPAM 分配特点是：

1. 子网中的 IP 地址按照节点来分割

    集群中拥有一个或者多个定义了 IP 子网号的 subnet 实例，以一个更小子网掩码长度为单位，把 subnet 分割出更小的 IP block 集合，依据
    IP 分配或者释放的实际情况，每个 node 都会获取到一个或者多个 IP block。

    这意味着两个特点：第一，每个 node 上的 IPAM 插件只需要在本地的 IP block 中分配和释放 IP 地址时，与其它 node 上的 IPAM 无 IP 分配冲突，IPAM 分配效率更高。
    第二，某个具体的 IP 地址跟随 IP block 集合，会相对固定的一直在某个 node 上被分配，没法随同 POD 一起被调度漂移。

2. IP 地址资源充沛

    众所周知，IPv6 地址资源非常充沛，而 IPv4 地址资源相比稀缺。因为"overlay 网络方案"下 POD 子网可以独立规划，
    只要能够合理利用 NAT 技术，kubernetes 单个集群可以拥有充沛的 IP 地址资源。
    因此，应用不会因为 IP 不够而启动失败，IPAM 组件面临的异常 IP 回收压力较小。

3. 没有应用" IP 地址固定"需求

    对于应用 IP 地址固定需求，有无状态应用和有状态应用的区别：对于 deployment 这类无状态应用，因为 POD name 会随着 POD 重启而变化，
    应用本身的业务逻辑也是无状态的，因此对于" IP 地址固定" 的需求，只能让所有 POD 副本固定在一个 IP 地址的集合内；对于 statefulset 这
    类有状态应用，因为 POD name 等信息都是固定的，应用本身的业务逻辑也是有状态的，因此对于" IP 地址固定"需求，要实现单个 POD 和具体 IP 地址的强绑定。

    在" overlay 网络方案"方案下，多是借助了 NAT 技术向集群外部暴露服务的入口和源地址，借助 DNS、clusterIP 等技术来实现集群东西向通信。
    其次，IPAM 的 IP block 方式把 IP 相对固定到某个节点上，而不能保证应用副本的跟随调度。
    因此，应用的" IP 地址固定"能力无用武之地，当前社区的主流 CNI 多数不支持" IP 地址固定"，或者支持方法较为简陋。

这个方案的优点是，无论集群部署在什么样的底层网络环境上，CNI 插件的兼容性都非常好，且都能够为 POD 提供子网独立、IP 地址资源充沛的网络。

这个方案的缺点是，对于一些传统应用的云化改造，提出了一些网络改造的挑战，包括 NAT 映射地址的适配和感知等。

### underlay 网络方案

本方案实现了 POD 共享宿主机网络，即 POD 直接获取宿主机网络中的 IP 地址，这样，应用可直接使用自己的 IP 地址进行东西向和南北向通信。

underlay 网络方案的实施，有两种典型的场景，一种是集群部署实施在"传统网络"上，一种是集群部署在 IAAS 环境上，例如公有云。以下总结了"传统网络场景"的 IPAM 特点：

1. 单个 IP 地址应该能够在任一节点上被分配

    这个需求有多方面的原因：随着数据中心的网络设备增加、多集群技术的发展，IPv4 地址资源稀缺，要求 IPAM 提高 IP 资源的使用效率；
    对于有" IP 地址固定"需求的应用，其 POD 副本可能会调度到集群的任意一个节点上，并且，在故障场景下还会发生节点间的漂移，要求 IP 地址一起漂移。

    因此，在集群中的任意一个节点上，一个 IP 地址应该具备能够被分配给 POD 使用的可能，而不同于"overlay 网络方案"中， IP 地址被 IP block 单位
    进行分割后，只能在某一个节点上被分配使用。

2. 同一应用的不同副本，能实现跨子网获取 IP 地址

    例如，一个集群中，宿主机1的区域只能使用子网 172.20.1.0/24，而宿主机2的区域只能使用子网 172.20.2.0/24， 在此背景下，
    当一个应用跨子网部署副本时，要求 IPAM 能够在不同的节点上，为应用分配出子网匹配的 IP 地址。

3. 应用 IP 地址固定

    很多传统应用在云化改造前，是部署在裸金属环境上的，服务之间的网络未引入 NAT 地址转换，微服务架构中需要感知对方的源 IP 或目的 IP ，
    并且，网络管理员也习惯了使用防火墙等手段来精细管控网络安全。

    因此，应用上云后，无状态应用希望能够实现 IP 范围的固定，有状态应用希望能够实现 IP 地址的唯一对应，这样，能够减少对微服务架构的改造工作。

4. 一个 POD 的多网卡获取不同子网的 IP 地址

    既然是对接 underlay 网络，POD 就会有多网卡需求，以使其通达不同的 underlay 子网，这要求 IPAM 能够给应用的不同网卡分配不同子网下的 IP 地址。

5. IP 地址冲突

    在 underlay 网络中，更加容易出现 IP 冲突，例如，POD 与集群外部的主机 IP 发生了冲突，与其它对接了相同子网的集群冲突，
    而 IPAM 组件很难感知外部这些冲突的 IP 地址，多需要借助 CNI 插件进行实时的 IP 冲突检测。

6. 已用 IP 地址的释放回收

    因为 underlay 网络 IP 地址资源的稀缺性，且应用有 IP 地址固定需求，所以，"应当"被释放的 IP 地址若未被 IPAM 组件回收，新启动的 POD 可能会因为缺少 IP 地址而失败。
    这就要求 IPAM 组件拥有更加精准、高效、及时的 IP 回收机制。

这个方案的优势有：无需网络 NAT 映射的引入，对应用的云化网络改造，提出了最大的便利；底层网络的火墙等设备，可对 POD 通信实现相对较为精细的管控；无需隧道技术，
网络通信的吞吐量和延时性能也相对的提高了。

这个方案的缺点有：IP 地址管理工作相对繁琐，对 IPAM 组件的实现挑战更多；POD 网络规划会受到底层网络的限制。

## 架构

关于 spiderpool 的设计架构，可参考 [设计](./docs/concepts/arch.md)

## 支持 CNI

任何支持第三方 IPAM 插件的 CNI 项目，都可以配合 spiderpool，例如：

* [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan)

* [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan)

* [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan)

* [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni)

* [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni)

* [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni)

## 快速开始

快速搭建 spiderpool，启动一个应用，可参考 [例子](./docs/usage/basic.md)

## 核心功能

* 多子网实例设计

    管理员可创建多个子网，不同应用可指定使用不同子网的 IP 地址，且 IPv4 和 IPv6 子网实例解耦，满足 underlay 网络的各种复杂规划。可参考 [例子](./docs/usage/???)

* 自动帮助应用 IP 地址固定，且 IP 地址的固定数量能够自动弹性扩缩

    对于应用 IP 地址固定的需求，当前开源社区，一些项目的做法是在应用的 annotaiton 中 hard code IP 地址，这导致 IP 地址冲突要人工保证、应用扩缩容容时 IP 地址管理成本高，
    这种做法容易出现运维事故。Spiderpool 提出了 CRD 化管理，解决了如上诸多问题，且让运维工作量最小化。

    对无状态应用，可实现自动固定 IP 地址范围，且能够自动跟随应用副本数进行 IP 资源扩缩容和删除。可参考 [例子](./docs/usage/???)

    对有状态应用，可自动为每个 POD 固定 IP 地址，并且也可固定整体的 IP 扩缩容范围。可参考 [例子](./docs/usage/???)

* 对于没有 IP 地址固定需求的应用，可共同使用一个共享 IP 池。可参考 [例子](./docs/usage/???)

* 对于一个跨子网部署的应用，支持为其不同副本分配不同子网的 IP 地址。可参考 [例子](./docs/usage/ippool-affinity-node.md)

* 应用可设置多个 IP 池，实现 IP 资源的备用效果。可参考 [例子](./docs/usage/ippool-multi.md)

* 设置全局的 IP 预留，使得集群不会分配出这些 IP 地址，这样能避免与集群外部的 IP 冲突。可参考 [例子](./docs/usage/reserved-ip.md)

* 能够为一个具备多网卡的 POD 分配不同子网下的 IP 地址。可参考 [例子](./docs/usage/multi-interfaces-annotation.md)

* 对第三方应用控制器的支持。可参考 [例子](./docs/usage/third-party-controller.md)

* IP 池可配置为全局可共享，也可实现同指定租户的绑定。可参考 [例子](./docs/usage/ippool-affinity-namespace.md)

* 合理的 IP 回收机制设计，可最大保证 IP 资源的可用性。可参考 ？？？

* 以上所有的功能，都能够在 ipv4-only、ipv6-only、dual-stack 场景下工作

## 其它功能

* 指标

* 支持 AMD64 和 ARM64

* 基于 webhook 校验，对于 CR 实例进行强校验，在管理员失误、并发操作等场景下，能够避免出现 IP 泄露、IP 冲突、格式错误等问题。

* 性能

## License

Spiderpool is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for the full license text.
