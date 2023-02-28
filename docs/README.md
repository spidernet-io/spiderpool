# Spiderpool

## Introduction

The Spiderpool is an IP Address Management (IPAM) CNI plugin that assigns IP addresses for kubernetes clusters.

Currently, it is under developing stage, not ready for production environment yet.

Any Container Network Interface (CNI) plugin supporting third-party IPAM plugins can use the Spiderpool,
such as [MacVLAN CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[VLAN CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan), [IPVLAN CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan) etc.
The Spiderpool also supports
[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni)
case to assign IP for multiple interfaces.
More CNIs will be tested for integration with Spiderpool.

## Why Spiderpool

Most overlay CNIs, like
[Cilium](https://github.com/cilium/cilium)
and [Calico](https://github.com/projectcalico/calico),
have a good implementation of IPAM, so the Spiderpool is not intentionally designed for these cases, but may be integrated with them.

The Spiderpool is specifically designed to use with underlay network, where administrators can accurately manage each IP.

Currently, the community already has some IPAM plugins such as [whereabout](https://github.com/k8snetworkplumbingwg/whereabouts), [kube-ipam](https://github.com/cloudnativer/kube-ipam),
[static](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/static),
[dhcp](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/dhcp), and [host-local](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/host-local),
but few of them could help solve complex underlay-network issues, so we decide to develop the Spiderpool.

BTW, there are also some CNI plugins that could work in the underlay mode, such as [kube-ovn](https://github.com/kubeovn/kube-ovn) and [coil](https://github.com/cybozu-go/coil).
But the Spiderpool provides lots of different features. See [Features](#features) for details.

## Features

The Spiderpool provides a large number of different features as follows.

* Based on CRD storage, all operation could be done with kubernetes API-server.

* Support for assigning IP addresses with three options: IPv4-only, IPv6-only, and dual-stack.

* Support for working on the clusters with three options: IPv4-only, IPv6-only, and dual-stack.

* Support for creating multiple ippools.
  Different namespaces and applications could monopolize or share an ippool.

* An application could specify multiple backup ippool resources in case IP addresses in an ippool are out of use. Therefore, you neither need to scale up the IP resources in a fixed ippool, nor need to modify the application yaml to change an ippool.

* Support to bind a range of IP addresses to a single application. No need to hard code an IP list in a deployment yaml, which is not easy to modify. With Spiderpool, you only need to set the selector field of ippool and scale up or down the IP resource of an ippool dynamically.

* Support for always assigning the same IP address to a StatefulSet pod.

* Different pods in a single controller could get IP addresses from
  different subnets for an application deployed in different subnets or zones.

* Administrator could safely edit ippool resources, where the Spiderpool will help validate the modification and prevent from data race.

* Collect resources in real time, especially for solving IP leakage or slow collection, which may make new pod fail to assign IP addresses.

* Support ranges of CNI plugin that supports third-party IPAM plugins. Especially, the Spiderpool could help much for CNI like [spiderflat](https://github.com/spidernet-io/spiderflat),
  [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
  [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
  [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
  [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
  [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni).

* Especially support for [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) case to assign IP for multiple interfaces.

* Have a good performance for assigning and collecting IP.

* Support to reserve IP that will not be assigned to any pod.

* Included metrics for looking into IP usage and issues.

* By CIDR Manager, it could automatically scale up and down the IP address of the ippool, to distribute IP resource more reasonable between ippool.

* Support for both ARM64 and ARM64.

## Components

Refer to [architecture](concepts/arch.md) for components.

## Installation

Refer to [installation](usage/install.md).

## Quick Start

Refer to [demo](usage/demo).

## Development

[Development guide](develop/pullrequest.md) is a reference point for development helper commands.

## License

Spiderpool is licensed under the Apache License, Version 2.0.
