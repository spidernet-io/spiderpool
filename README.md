# Spiderpool

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

***

**English** | [**简体中文**](./README-zh_CN.md)

## Introduction

Spiderpool:an IP Address Management (IPAM) CNI plugin of Kubernetes for managing static ip for underlay network. Spiderpool provides kinds of complete solutions with other CNI projects compatible with third-party IPAM plugins.

Why Spiderpool? There has not yet been a comprehensive, user-friendly and intelligent open source solution for what underlay networks' IPAMs needs:

* IPAM projects are rare in the open source community, let alone CRD-based projects fulfilling the IPAM needs of underlay networks.

* The data center of some special institutions having high demands for network security employs firewalls to control underlay network traffic. However, unfixed Pod IPs might lead to the high operation cost for firewall policies.

* Scarcity of IPv4 addresses in data centers requires efficient and timely allocation and release of Pod IPs, without IP conflicts or leaks.

* Different PODs of a deployment need to be assigned IP addresses of multiple underlay subnets when different nodes under the same cluster are distributed across network zones, which has not yet been supported by the open source community.

* CNIs are not able to automatically coordinate policy-based routing between multiple NICs when a POD is connected to multiple underlay NICs, perhaps resulting in inconsistent network requests and reply data of forwarding routing, and failed network access.

This is where Spiderpool comes to play to eliminate the complexity of allocating IP addresses to underlay networks. With the hope of the operations of IP allocation being as simple as some overlay-network CNIs, Spiderpool will be a new IPAM alternative for open source enthusiasts.

## IPAM for underlay and overlay network

There are two technologies in cloud-native networking: "overlay network" and "underlay network".
Despite no strict definition for underlay and overlay networks in cloud-native networking, we can simply abstract their characteristics from many CNI projects. The two technologies meet the needs of different scenarios.
 Spiderpool is designed for underlay networks, and the following comparison of the two solutions can better illustrate the features and usage scenarios of Spiderpool.

For the detail, it could refer to [IPAM description of underlay network](IPAM for underlay and overlay network).



## Architecture

架构图

Spiderpool mainly provides two CNI plugins:

* spiderpool plugin

    It is an IPAM plugin, and assign IP address for one or more NIC of underlay CNI. 

* coordenator plugin: 

    It is a meta plugin, it could do kinds of supplementary things:

  * tune routes for multiple NIC

  * detect IP conflict

  * detect reachability of gateway

  * set the MAC address of NIC to a fixed format

## Use Cases

Spiderpool could have two kinds of use cases :

* pod with underlay NICs

  For this use case, the cluster could use one or more underlay CNI to run pods.

  When one or more underlay NIC in a pod, spiderpool could help assign IP address, tune routes, connect the pod and local node, detect IP conflict etc.

* pod with one overlay NIC and more underlay NICs

  For this use case, the cluster could use one overlay CNI and other underlay CNI to run pods.

  When one or more NIC of different NIC in a pod, spiderpool could help assign IP address, tune routes, connect the pod and local node, detect IP conflict etc.

## Supported CNIs

Any CNI project compatible with third-party IPAM plugins, can work well with spiderpool, such as:

[macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan), 
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan), 
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan), 
[sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni), 
[ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni), 
[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), 
[calico CNI](https://github.com/projectcalico/calico), 
[weave CNI](https://github.com/weaveworks/weave)

Additionally, Spiderpool could help some CNI addons be able to access clusterIP and pod-healthy check, like [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan), [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni), [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni).

## Quick start

If you want to start some Pods with Spiderpool in minutes, refer to [Quick start](./docs/usage/install/install.md).

## Major features

* Multiple subnet objects

    The administrator can create multiple subnets objects mapping to each underlay CIDR, and applications can be assigned IP addresses within different subnets. to meet the complex planning of underlay networks. See [example](./docs/usage/multi-interfaces-annotation.md) for more details.

* Automatical ippool for applications needing static ip

    To realize static IP addresses, some open source projects need hardcode IP addresses in the application's annotation, which is prone to operations accidents, manual operations of IP address conflicts, higher IP management costs caused by application scalability.
    Spiderpool could automatically create, delete, scale up and down a dedicated ippool with static IP address just for one application, which could minimize operation efforts.

  * For stateless applications, the IP address range can be automatically fixed and IP resources can be dynamically scaled according to the number of application replicas. See [example](./docs/usage/spider-subnet.md) for more details.

  * For stateful applications, IP addresses can be automatically fixed for each POD, and the overall IP scaling range can be fixed as well. And IP resources can be dynamically scaled according to the number of application replicas. See [example](./docs/usage/statefulset.md) for more details.
    
  * The dedicated ippool could have keep some redundant IP address, which supports application to performance a rolling update when creating new pods. See [example](./docs/usage/????) for more details.

  * Support for third-party application controllers. See [example](./docs/usage/third-party-controller.md) for details
    
* Manual ippool for applications needing static ip but the administrator expects specify IP address by hand. See [example](./docs/usage/ippool-affinity-pod.md) for details

* For applications not requiring static IP addresses, they can share an IP pool. See [example](./docs/usage/ippool-affinity-pod.md#shared-ippool) for details

* For one application with pods running on nodes accessing different underlay subnet, spiderpool could assign IP addresses within different subnets. See [example](./docs/usage/ippool-affinity-node.md) for details

* Multiple IP pools can be set for a pod for the usage of backup IP resources. See [example](./docs/usage/ippool-multi.md) for details

* Set global reserved IPs that will not be assigned to Pods, it can avoid to misuse IP address already used by other network hosts. See [example](./docs/usage/reserved-ip.md) for details

* when assigning multiple NICs to a pod with [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), spiderpool could specify different subnet for each NIC. See [example](./docs/usage/multi-interfaces-annotation.md) for details

* IP pools can be shared by whole cluster or bound to a specified namespace. See [example](./docs/usage/ippool-affinity-namespace.md) for details

* An additional plugin [veth](https://github.com/spidernet-io/plugins) provided by spiderpool has features:

* help some CNI addons be able to access clusterIP and pod-healthy check , such as [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan), 
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan), 
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan), 
[sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni), 
[ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni). See [example](./docs/usage/get-started-macvlan.md) for details.

* help coordinate routes of each NIC, for pods who has multiple NICs assigned by [Multus](https://github.com/k8snetworkplumbingwg/multus-cni). See [example](./docs/usage/multi-interfaces-annotation.md) for details

* Private IPv4 address is rare, spiderpool provides a reasonable IP recycling mechanism, especially for running new pods when nodes or old pods are abnormal. See [example](./docs/usage/gc.md) for details

* The administrator could specify customized route. See [example](./docs/usage/route.md) for details

* By comparison with other open source projects in the community, outstanding performance for assigning and releasing Pod IPs is showcased in the [test report](docs/usage/performance.md) covering multiple scenarios of IPv4 and IPv6:

  * Enable fast allocation and release of static IPs for large-scale creation, restart, and deletion of applications

  * Enable applications to quickly obtain IP addresses for self-recovery after downtime or a cluster host reboot

* All above features can work in ipv4-only, ipv6-only, and dual-stack scenarios. See [example](./docs/usage/ipv6.md) for details

## Other features

* [Metrics](./docs/concepts/metrics.md)

* Support AMD64 and ARM64

* lots of design can avoid IP leaks, IP conflicts, in case of administrator's fault, concurrent operations and so on.

## License

Spiderpool is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for the full license text.
