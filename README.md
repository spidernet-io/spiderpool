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


## Status

Currently, the Spiderpool is under beta stage, not ready for production environment yet.

## Introduction

The Spiderpool is an IP Address Management (IPAM) CNI plugin that assigns IP addresses for kubernetes clusters.

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
have a good implementation of IPAM, so the Spiderpool is not intentionally designed for these cases, but maybe integrated with them.

The Spiderpool is intentionally designed to use with underlay network, where administrators can accurately manage each IP.

Currently, in the community, the IPAM plugins such as [whereabout](https://github.com/k8snetworkplumbingwg/whereabouts), [kube-ipam](https://github.com/cloudnativer/kube-ipam),
[static](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/static),
[dhcp](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/dhcp), and [host-local](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/host-local),
few of them could help solve complex underlay-network issues, so we decide to develop the Spiderpool.

BTW, there are also some CNI plugins that could work on the underlay mode, such as [kube-ovn](https://github.com/kubeovn/kube-ovn) and [coil](https://github.com/cybozu-go/coil).
But the Spiderpool provides lots of different features, you could see [Features](#features) for details.

## Features

The Spiderpool provides a large number of different features as follows.

* Support ranges of CNI plugin who supports third-party IPAM plugins. Especially, the Spiderpool could help much for CNI like [spiderflat](https://github.com/spidernet-io/spiderflat),
  [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
  [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
  [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
  [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
  [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni).
  
* Especially, support multiple interfaces for [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), a pod could specify different ippool for each interface.

* Support for assigning IP addresses with three options: IPv4-only, IPv6-only, and dual-stack.

* Support for working on the clusters with three options: IPv4-only, IPv6-only, and dual-stack.

* Support for creating multiple ippools.
  Different namespaces and applications could monopolize or share an ippool.

* An application could specify multiple backup ippool resources, in case that IP addresses in an ippool are out of use. Therefore, you neither need to scale up the IP resources in a fixed ippool, nor need to modify the application yaml to change a ippool.

* An applications could bind range of fixed IP address. No need to hard code an IP list in deployment yaml, which is not easy to modify. With Spiderpool, you only need to set the selector field of ippool and scale up or down the IP resource of an ippool dynamically.

* Support Statefulset pod who will be always assigned same IP addresses.

* For an application deployed in different subnets or zones, its pods could get IP addresses from
  different subnets.

* Collect resources in real time, especially for solving IP leakage or slow collection, which may make new pod fail to assign IP addresses.

* By SpiderSubnet feature, it could automatically create new ippool for application who needs fixed IP address, and retrieve the ippool when application is deleted. Especially, the created ippool could automatically scale up or down the IP number when the replica number of application changes. That could reduce the administrator workload.

* Support to reserve IP who will not be assigned to any pod.

* Included metrics for looking into IP usage and issues.

* Have a good performance for assigning and collecting IP.

* Based on CRD storage, all operation could be done with kubernetes API-server.

* Administrator could safely edit kinds of spiderpool CRD resources, the Spiderpool will help validate the modification and prevent from kinds of conflict and mistake.

* Support for both AMD64 and ARM64.

## Components

Refer to [architecture](docs/concepts/arch.md) for components.

## Installation


Refer to [installation](./docs/usage/install.md).

## Quick Start

Refer to [demo](./docs/usage/basic.md).

## Development


[Development guide](docs/develop/pullrequest.md) is a reference point for development helper commands.

## License

Spiderpool is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for the full license text.
