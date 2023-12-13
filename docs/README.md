#

![Spiderpool](./images/spiderpool.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/spidernet-io/spiderpool)](https://goreportcard.com/report/github.com/spidernet-io/spiderpool)
[![CodeFactor](https://www.codefactor.io/repository/github/spidernet-io/spiderpool/badge)](https://www.codefactor.io/repository/github/spidernet-io/spiderpool)
[![codecov](https://codecov.io/gh/spidernet-io/spiderpool/branch/main/graph/badge.svg?token=YKXY2E4Q8G)](https://codecov.io/gh/spidernet-io/spiderpool)
[![Auto Version Release](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-version-release.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-version-release.yaml)
[![Auto Nightly CI](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/6009/badge)](https://bestpractices.coreinfrastructure.org/projects/6009)
[![Nightly K8s Matrix](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-diff-k8s-ci.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-diff-k8s-ci.yaml)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/7e54bfe38fec206e7710c74ad55a5139/raw/spiderpoolcodeline.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/e1d3c092d1b9f61f1c8e36f09d2809cb/raw/spiderpoole2e.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/cd9ef69f5ba8724cb4ff896dca953ef4/raw/spiderpooltodo.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/38d00a872e830eedb46870c886549561/raw/spiderpoolperformance.json)

**English** | [**简体中文**](./README-zh_CN.md)

## Introduction

> Spiderpool is currently a project at the [CNCF Landscape](https://landscape.cncf.io/card-mode?category=cloud-native-network&grouping=category) level.

Spiderpool is an underlay and RDMA network solution for the Kubernetes. It enhances the capabilities of [Macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
[SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni), It satisfies networking requirements including but not limited to the following:

- Pods dynamically connect to different Underlay networks.
- Overlay and Underlay need to coexist in a Kubernetes cluster.
- Underlay CNIs should have access to Services as well as address Pod health check issues.
- In the case of cross-data-center network isolation, the problem of non-communication in a multi-cluster network arises.
- Users operating in different environments (bare metal, virtual machines, or public clouds) require a unified Underlay CNI solution.
- For latency-sensitive applications, users urgently need to reduce network latency.

Spiderpool enables the utilization of underlay network solutions in **bare metal, virtual machine, and public cloud environments**. and delivers exceptional network performance, particularly benefiting network I/O-intensive and low-latency applications like **storage, middleware, and AI**.
It could refer to [website](https://spidernet-io.github.io/spiderpool/) for more details.

> Spiderpool is currently a project at the [CNCF Landscape](https://landscape.cncf.io/card-mode?category=cloud-native-network&grouping=category) level.

## Spiderpool Functionality Overview

<div style="text-align:center">
  <img src="./images/arch.png" alt="Your Image Description">
</div>

- Simplified installation and usage

    Spiderpool simplifies the installation process by eliminating the need for manually installing multiple components like [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni). It provides streamlined installation procedures, encapsulates relevant CRDs, and offers comprehensive documentation for easy setup and management.

- CRD-based dual-stack IPAM

    Spiderpool provides exclusive and shared IP address pools, supporting various affinity settings. It allows configuring specific IP addresses for stateful applications like middleware and kubevirt, while enabling fixed IP address ranges for stateless ones. Spiderpool automates the management of exclusive IP pools, ensuring excellent IP reclamation to avoid IP leakage. In additions, it owns [wonderful IPAM performance](./concepts/ipam-performance.md) .

- Make underlay and overlay network can coexist in a Kubernetes cluster

    Pods can have multiple CNI interfaces by attaching multiple underlay CNI interfaces or attaching one overlay CNI interface along with multiple underlay CNI interfaces. Spiderpool can customize different IP addresses for multiple underlay CNI interfaces, coordinate policy routing among all interfaces to ensure consistent data paths for requests and responses, avoiding packet loss. This enables the coexistence of overlay and multiple underlay networks in a Kubernetes cluster It could strengthen [cilium](https://github.com/cilium/cilium), [calico](https://github.com/projectcalico/calico), [kubevirt](https://github.com/kubevirt/kubevirt) .

- Enhanced network connectivity

    Spiderpool establishes seamless connectivity between Pods and host machines, ensuring smooth functioning of Pod health checks. It enables Pods to access services through kube-proxy or eBPF-based kube-proxy replacement. Additionally, it supports advanced features like IP conflict detection and gateway reachability checks. The network of Multi-cluster could be connected by a same underlay network, or [Submariner](https://github.com/submariner-io/submariner) .

- eBPF enhancements

    The eBPF-based kube-proxy replacement significantly accelerates service access, while socket short-circuiting technology improves local Pod communication efficiency within the same node. Compared with kube-proxy manner, [the improvement of the performance is Up to 25% on network delay, up to 50% on network throughput](./concepts/io-performance.md).

- RDMA support

    Spiderpool provides RDMA solutions based on RoCE and InfiniBand technologies.

- Dual-stack network support

    Spiderpool supports IPv4-only, IPv6-only, and dual-stack environments.

- Good network performance of latency and throughput

    Spiderpool performs better than overlay CNI on network latency and throughput, referring to [performance report](./concepts/io-performance.md)

- Metrics

## Why does Spiderpool select macvlan, ipvlan, and SR-IOV as datapaths?

- macvlan, ipvlan, and SR-IOV is crucial for supporting RDMA network acceleration. RDMA significantly enhances performance for AI applicaitons, latency-sensitive and network I/O-intensive applications, surpassing overlay network solutions in terms of network performance.

- Unlike CNI solutions based on veth virtual interfaces, underlay networks eliminate layer 3 network forwarding on the host, avoiding tunnel encapsulation overhead. This translates to excellent network performance with high throughput, low latency, and reduced CPU utilization for network forwarding.

- Connecting seamlessly with underlay layer 2 VLAN networks enables both layer 2 and layer 3 communication for applications. It supports multicast and broadcast communication, while allowing packets to be controlled by firewalls.

- Data packages carry the actual IP addresses of Pods, enabling direct north-south communication based on Pod IPs. This connectivity across multi-cloud networks enhances flexibility and ease of use.

- Underlay CNI can create virtual interfaces using different parent network interfaces on the host, providing isolated subnets for applications with high network overhead, such as storage and observability.

## Spiderpool Architecture

Spiderpool has a well-defined architectural design, including the following components:

- _Spiderpool-controller_: A set of Deployments that interact with the API-Server, managing multiple Custom Resource Definition (CRD) resources such as [SpiderIPPool](../reference/crd-spiderippool.md),[SpiderSubnet](../reference/crd-spidersubnet.md),[SpiderMultusConfig](../reference/crd-spidermultusconfig.md), etc. It implements validation, creation, and status management for these CRDs. Additionally, it responds to requests from Spiderpool-agent Pods, handling functions like allocation, release, recycling, and managing automatic IP pools.

- _Spiderpool-agent_: A set of daemonsets running on each node, aiding in the installation of binaries such as Multus, Coordinator, IPAM, CNI, etc., on every node. It responds to CNI requests for IP allocation during Pod creation, interacts with Spiderpool-controller to complete Pod IP allocation and release. Simultaneously, it collaborates with the Coordinator, assisting in the implementation of configuration synchronization for coordinator plugins.

- _CNI Plugins_: It includes but is not limited to  plugins such as Multus, Macvlan, IPVlan, Sriov-CNI, Rdma-CNI, Coordinator, Ifacer, etc.

- _[sriov-network operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator)_

- _[RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)_

For more details please see[Spiderpool Architecture](./concepts/arch.md).

## RoadMap

| Features                               | macvlan    | ipvlan            | SR-IOV      |
|----------------------------------|------------|-------------------|-------------|
| Service By Kubeproxy             | Beta       | Beta              | Beta        |
| Service By Kubeproxy Replacement | Alpha      | Alpha             | Alpha       |
| Network Policy                   | In-plan    | Alpha             | In-plan     |
| Bandwidth                        | In-plan    | Alpha             | In-plan     |
| RDMA                             | Alpha      | Alpha             | Alpha       |
| IPAM                             | Beta       | Beta              | Beta        |
| Multi-Cluster                    | Alpha    | Alpha             | Alpha     |
| Egress Policy                    | Alpha      | Alpha             | Alpha       |
| Multiple NIC And Routing Coordination                         | Beta       | Beta              | Beta        |
| Scenarios                             | Bare metal | Bare metal and VM | Bare metal  |

For detailed information about all the planned features, please refer to the [roadmap](./develop/roadmap.md).

## Quick start

Refer to [Quick start](./usage/install/get-started-kind.md) to explore Spiderpool quickly.

Refer to [Usage Index](./usage/readme.md) for usage details.

## Blogs

Refer to [Blogs](./concepts/blog.md)

## Governance

The project is governed by a group of [Maintainers and Committers](./AUTHORS). How they are selected and govern is outlined in our [governance document](./develop/CODE-OF-CONDUCT.md).

## Adopters

A list of adopters who are deploying Spiderpool in production, and of their use cases, can be found in [file](./USERS.md).

## Contribution

Refer to [Contribution](./develop/contributing.md) to join us for developing Spiderppol.

## Contact Us

If you have any questions, please feel free to reach out to us through the following channels:

- Slack: join the [#Spiderpool](https://cloud-native.slack.com/messages/spiderpool) channel on CNCF Slack by requesting an **[invitation](https://slack.cncf.io/)** from CNCF Slack. Once you have access to CNCF Slack, you can join the Spiderpool channel.

- Email: refer to the [MAINTAINERS.md](https://github.com/spidernet-io/spiderpool/blob/main/MAINTAINERS.md)  to find the email addresses of all maintainers. Feel free to contact them via email to report any issues or ask questions.

- Community Meeting: Welcome to our [community meeting](https://docs.google.com/document/d/1tpNzxRWOz9-jVd30xGS2n5X02uXQuvqJAdNZzwBLTmI/edit?usp=sharing) held on the 1st of every month. Feel free to join and discuss any questions or topics related to Spiderpool.

- WeChat group: scan the QR code below to join the Spiderpool technical discussion group and engage in further conversations with us.

![Wechat QR-Code](./images/wechat.png)

## License

Spiderpool is licensed under the Apache License, Version 2.0.
See [LICENSE](./LICENSE) for the full license text.

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="300"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="350"/>
<br/><br/>
Spiderpool enriches the <a href="https://landscape.cncf.io/?selected=spiderpool">CNCF Cloud Native Landscape</a>.
</p>
