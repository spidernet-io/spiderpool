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

Spiderpool is a [CNCF Landscape Level Project](https://landscape.cncf.io/card-mode?category=cloud-native-network&grouping=category).

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

### Overlay network solution

These solutions implement the decoupling of POD network and host network, such as [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium) and other CNI plugins. Typically, They use tunnel technology such as vxlan to build an overlay network plane, and use NAT technology for north-south traffic.

These IPAM solutions has some characteristics:

1. divide pod subnet into node-based IP block

    In terms of a smaller subnet mask, the pod subnet is divided into smaller IP blocks, and each node is assigned one or more IP blocks depending on the actual IP allocation account.

    First, since the IPAM plugin on each node only needs to allocate and release IP addresses in the local IP block, there is no IP allocation conflict with IPAM on other nodes, and achieve more efficient allocation.
    Second, a specific IP address follows an IP block and is allocated within one node all the time, so it cannot be assigned on other nodes together with a bound POD.

2. Sufficient IP address resources

    subnets not overlapping with any CIDR, could be used by the cluster, so the cluster have enough IP address resources as long as NAT technology is used in an appropriate manner. As a result, IPAM components face less pressure to reclaim abnormal IP address.

3. No requirement for static IP addresses

    For the static IP address requirement, there is a difference between stateless application and stateful application. Regarding stateless application like deployment, the POD's name will change when the POD restarts, the business logic of the application itself is stateless, so static IP addresses means that all the POD replicas are fixed in a set of IP addresses; for stateful applications such as statefulset, considering both the fixed information including POD's names and stateful business logic, the strong binding of one POD and one specific IP address needs to be implemented for static IP addresses.

    The "overlay network solution" mostly exposes the ingress and source addresses of services to the outside of the cluster with the help of NAT technology, and realizes the east-west communication through DNS, clusterIP and other technologies.
    In addition, although the IP block of IPAM fixes the IP to one node, it does not guarantee the application replicas to follow the scheduling.Therefore, there is no scope for the static IP address capability. Most of the mainstream CNIs in the community have not yet supported "static IP addressed", or support it in a rough way.

The advantage of the "overlay network solution" is that the CNI plugins are highly compatible with any underlying network environment, and can provide independent subnets with sufficient IP addresses for PODs.

### Underlay network solution

This solution shares node's network for PODs, which means PODs can directly obtain IP addresses in the node network. Thus, applications can directly use their own IP addresses for east-west and north-south communications.

There are two typical scenarios for underlay network solutions：clusters deployed on a "legacy network" and clusters deployed on an IAAS environment, such as a public cloud. The following summarizes the IPAM characteristics of the "legacy network scenario":

1. An IP address able to be assigned to any node

    As the number of network devices in the data center increases and multi-cluster technology evolves, IPv4 address resources become scarce, thus requiring IPAM to improve the efficiency of IP usage.
    As the POD replicas of the applications requiring "static IP addresses" could be scheduled to any node in the cluster and drift between nodes, IP addresses might drift together.

    Therefore, an IP address should be able to be allocated to a POD on any node.

2. Different replicas within one application could obtain IP addresses across subnets

    Take as an example one node could access subnet 172.20.1.0/24 while another node just only access subnet 172.20.2.0/24. In this case, when the replicas within one application need be deployed across subnets, IPAM is required to be able to assign subnet-matched IP addresses to the application on different nodes.

3. Static IP addresses

    For some traditional applications, the source IPs or destination IPs needs to be sensed in the microservice. And network admins are used to enabling fine-grained network security control via firewalls and other means.

    Therefore, in order to reduce the transformation chores after the applications move to the kubernetes, applications need static IP address.

4. Pods with Multiple NICs need IP addresses of different underlay subnets

    Since the POD is connected to an underlay network, it has the need for multiple NICs to reach different underlay subnets.

5. IP conflict

    Underlay networks are more prone to IP conflicts. For instance, PODs conflict with host IPs outside the cluster, or conflict with other clusters under the same subnet. But it is difficult for IPAM to discover these conflicting IP addresses externally unless CNI plugins are involved for real-time IP conflict detection.

6. Release and recover IP addresses

    Because of the scarcity of IP addresses in underlay networks and the static IP address requirements of applications, a newly launched POD may fail due to the lack of IP addresses owing to some IP addresses not released by abnormal Pods.
    This requires IPAMs to have a more accurate, efficient and timely IP recovery mechanism.

The advantages of the underlay network solution include: no need for network NAT mapping, which makes cloud-based network transformation for applications way more convenient; the underlying network firewall and other devices can achieve relatively fine control of POD communication; no tunneling technology contributes to improved throughput and latency performance of network communications.

## Architecture

For the architecture of spiderpool, refer to [Architecture](./docs/concepts/arch.md).

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

If you want to start some Pods with Spiderpool in minutes, refer to [Quick start](./docs/usage/install.md).

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
