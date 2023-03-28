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
***

**English** | [**简体中文**](./README-zh_CN.md)

## Introduction

Spiderpool is a Kubernetes IP Address Management (IPAM) CNI plugin designed to provide the ability of IP address management for underlay networks. Spiderpool can work well with any CNI project that is compatible with third-party IPAM plugins.

Why Spiderpool? Given that there has not yet been a comprehensive, user-friendly and intelligent open source solution for what underlay networks' IPAMs need, Spiderpool comes in to eliminate the complexity of allocating IP addresses to underlay networks.
With the hope of the operations of IP allocation being as simple as some overlay network models, Spiderpool supports many features, such as static application IP addresses, dynamic scalability of IP addresses, multi-NIC, dual-stack support, etc.
Hopefully, Spiderpool will be a new IPAM alternative for open source enthusiasts.

## IPAM for underlay and overlay network

There are two technologies in cloud-native networking: "overlay network" and "underlay network".
Despite no strict definition for underlay and overlay networks in cloud-native networking, we can simply abstract their characteristics from many CNI projects. The two technologies meet the needs of different scenarios.
 Spiderpool is designed for underlay networks, and the following comparison of the two solutions can better illustrate the features and usage scenarios of Spiderpool.

### Overlay network solution

These solutions implement the decoupling of POD network and host network, such as [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium) and other CNI plugins. They either use tunneling technologies such as vxlan to build an overlay network plane, and then employ NAT for north-south traffic, or they use BGP routing protocols to open up the routes of the host network so that the host network can forward the POD's East-west and north-south traffic.

The characteristics of these IPAM solutions for overlay networks:

1. Group IP addresses in a subnet by node

    The cluster has one or more instances of subnet with IP subnet ID defined. In terms of a smaller subnet mask, the subnet is divided into smaller chunks - called IP blocks, and each node is assigned one or more IP blocks depending on the actual IP allocation or release.

    This means:
    First, since the IPAM plugin on each node only needs to allocate and release IP addresses in the local IP block, there is no IP allocation conflict with IPAM on other nodes, and achieve more efficient allocation.
    Second, a specific IP address follows an IP block and is allocated within a node all the time, and cannot be scheduled with the POD.

2. Sufficient IP address resources

    As we all know, IPv6 address resources are abundant while IPv4 addresses are scarce. Because POD subnets can be planned independently under the "overlay network solution", a kubernetes cluster can have enough IP address resources as long as NAT technology is used in an appropriate manner. As a result, applications do not fail to start due to insufficient IPs, and IPAM components face less pressure to recover abnormal IPs.

3. No requirement for static application IP addresses

    For the static IP address requirement, there is a difference between stateless application and stateful application. Regarding stateless application like deployment, the POD's name will change when the POD restarts. Moreover, the business logic of the application itself is stateless, so static IP addresses are enabled only if all the POD replicas are fixed in a set of IP addresses; for stateful applications such as statefulset, considering both the fixed information including POD's names and stateful business logic, the strong binding of one POD and specific IP addresses needs to be implemented for static IP addresses.

    The "overlay network solution" mostly exposes the ingress and source addresses of services to the outside of the cluster with the help of NAT technology, and realizes the east-west communication through DNS, clusterIP and other technologies.
    In addition, although the IP block of IPAM fixes the IP to a node, it does not guarantee the application replicas to follow the scheduling.Therefore, there is no scope for the static IP address capability. Most of the mainstream CNIs in the community have not yet supported "static IP addressed", or support it in a rough way.

The advantage of the "overlay network solution" is that the CNI plugins are highly compatible with any underlying network environment the cluster is deployed on, and can provide subnet independent networks with sufficient IP addresses for PODs.

The disadvantage is that it poses network upgrading challenges for the cloud transformation of some traditional applications, including the adaptation and awareness of NAT mapped addresses.

### Underlay network solution

This solution implements a shared host network for PODs, which means PODs can directly obtain IP addresses in the host network. Thus, applications can directly use their own IP addresses for east-west and north-south communications.

There are two typical scenarios for underlay network solutions：clusters deployed on a "legacy network" and clusters deployed on an IAAS environment, such as a public cloud. The following summarizes the IPAM characteristics of the "legacy network scenario":

1. An IP address should be able to be assigned to any node

    This requirement arises for multiple reasons:
    As the number of network devices in the data center increases and multi-cluster technology evolves, IPv4 address resources become scarce, thus requiring IPAM to improve the efficiency of IP usage.
    As the POD replicas of the applications requiring "static IP addresses" might be scheduled to any node in the cluster and drift between nodes in failure scenarios, IP addresses might drift together.

    Therefore, an IP address should be able to be allocated to a POD on any node, unlike the "overlay network solution" where IP addresses are divided in IP blocks and can only be assigned on one node.

2. Different replicas of the same application can obtain IP addresses across subnets

    Take as an example a cluster whose Host 1 can only use subnet 172.20.1.0/24 while Host 2 is under subnet 172.20.2.0/24. In this case, when the replicas of an application is deployed across subnets, IPAM is required to be able to assign subnet-matched IP addresses to the application on different nodes.

3. Static IP addresses

    Many traditional applications are deployed on bare metal before cloud transformation. Since NAT address translation is not available in the network between services, the source IPs or destination IPs needs to be sensed in the microservice architecture. And network admins are used to enabling fine-grained network security control via firewalls and other means.

    Therefore, in order to reduce the transformation chores of the microservice architecture after the applications move to the cloud, the stateless application need a fixed IP range and the stateful hope a static IP address.

4. Multiple NICs of a POD get IP addresses of different subnets

    Since the POD is connected to an underlay network, it has the need for multiple NICs to reach different underlay subnets, which requires IPAM to be able to assign IP addresses under different subnets to multiple NICs of the application.

5. IP conflict

    Underlay networks are more prone to IP conflicts. For instance, PODs conflict with host IPs outside the cluster, or conflict with other clusters under the same subnet. But it is difficult for IPAM to discover these conflicting IP addresses externally unless CNI plugins is involved for real-time IP conflict detection.

6. Release and recovery IP addresses

    Because of the scarcity of IP addresses in underlay networks and the static IP address requirements of applications, a newly launched POD may fail due to the lack of IP addresses if the IP addresses that "should" have been released are not recovered by IPAMs.
    This requires IPAMs to have a more accurate, efficient and timely IP recovery mechanism.

The advantages of the underlay network solution include: no need for network NAT mapping, which makes cloud-based network transformation for applications way more convenient; the underlying network firewall and other devices can achieve relatively fine control of POD communication; no tunneling technology contributes to improved throughput and latency performance of network communications.

The disadvantages are: IP address management is relatively cumbersome and more challenging for the implementation of IPAMs; POD network planning will be limited by the underlying network.

## Architecture

For the architecture of spiderpool, refer to [Architecture](./docs/concepts/arch.md).

## Supported CNIs

Any CNI project that is compatible with third-party IPAM plugins can work well with spiderpool, such as:

* [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan)

* [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan)

* [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan)

* [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni)

* [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni)

* [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni)

## Getting Started

If you want to start some Pods with Spiderpool in minutes, refer to [Getting Started](./docs/usage/getting-started.md).

## Major features

* Multiple subnet instances

    Admins can create multiple subnets, and different applications can be assigned different IP addresses of different subnets. IPv4 and IPv6 subnet instances are decoupled for the complex planning of underlay networks. See [example](./docs/usage/????) for more details.

* Automatic implementation of static IP addresses and dynamic scalability of IP count

    To realize static IP addresses, current open source projects hardcode IP addresses in the application's annotation, which is prone to operations accidents, manual operations of IP address conflicts as well as higher IP management costs caused by application scalability.
    Spiderpool provides CRD-based management to solve the above problems and minimize operation efforts.

    For stateless applications, the IP address range can be automatically fixed and IP resources can be dynamically scaled according to the number of application replicas. See [example](./docs/usage/????) for more details.

    For stateful applications, IP addresses can be automatically fixed for each POD, and the overall IP scaling range can be fixed as well. See [example](./docs/usage/????) for more details.

* Applications not requiring static IP addresses can share an IP pool. See [example](./docs/usage/????) for details

* Different replicas of applications can be assigned IP addresses of different subnets. See [example](./docs/usage/ippool-affinity-node.md) for details

* Multiple IP pools can be set in an application for backup IP resources. See [example](./docs/usage/ippool-multi.md) for details

* Set global reserved IPs that will not be allocated, which can avoid IP conflicts with IPs outside the cluster. See [example](./docs/usage/reserved-ip.md) for details

* Allocate IP addresses under different subnets to a POD with multiple NICs. See [example](./docs/usage/multi-interfaces-annotation.md) for details

* Support for third-party application controllers. See [example](./docs/usage/third-party-controller.md) for details

* IP pools can be shared globally or bound to a specified tenant. See [example](./docs/usage/ippool-affinity-namespace.md) for details

* Provide a sound IP recycling mechanism to maximize the usage of IP resources. See [example](./docs/usage/????) for details

* All the above features can work in ipv4-only, ipv6-only, and dual-stack scenarios

## Others

* Metrics

* Support AMD64 and ARM64

* Provide strong validation for CR instances based on validating webhooks, which can avoid IP leaks, IP conflicts, format errors, etc. in case of admin's errors, concurrent operations and so on.

* Performance

## License

Spiderpool is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for the full license text.
