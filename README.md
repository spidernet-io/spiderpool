# spider pool

[![Go Report Card](https://goreportcard.com/badge/github.com/spidernet-io/spiderpool)](https://goreportcard.com/report/github.com/spidernet-io/spiderpool)
[![CodeFactor](https://www.codefactor.io/repository/github/spidernet-io/spiderpool/badge)](https://www.codefactor.io/repository/github/spidernet-io/spiderpool)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=spidernet-io_spiderpool&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=spidernet-io_spiderpool)
[![codecov](https://codecov.io/gh/spidernet-io/spiderpool/branch/main/graph/badge.svg?token=YKXY2E4Q8G)](https://codecov.io/gh/spidernet-io/spiderpool)
[![Auto Release Version](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-release.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-release.yaml)
[![Auto Nightly CI](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml/badge.svg)](https://github.com/spidernet-io/spiderpool/actions/workflows/auto-nightly-ci.yaml)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/6009/badge)](https://bestpractices.coreinfrastructure.org/projects/6009)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/7e54bfe38fec206e7710c74ad55a5139/raw/spiderpoolcodeline.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/93b7ba26a4600fabe100ff640f9b3bd3/raw/spiderpoolcomment.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/e1d3c092d1b9f61f1c8e36f09d2809cb/raw/spiderpoole2e.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/cd9ef69f5ba8724cb4ff896dca953ef4/raw/spiderpooltodo.json)
![badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/38d00a872e830eedb46870c886549561/raw/spiderpoolperformance.json)

## Status

Currently, the spiderpool is under beta stage, not ready for production environment

## Introduction

The Spiderpool is an IP Address Management (IPAM) CNI plugin that assigns IP addresses for kubernetes cluster.

Currently, it is under developing stage, not ready for production environment.

Any CNI (Container Network Interface) plugin supporting third-party IPAM plugin, can use the Spiderpool,
such as [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan), [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan) etc.
The Spiderpool also support [multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) case to assign IP for multiple interfaces.
The Spiderpool will be tested with more CNI for integration.

## Why Spiderpool

For most overlay CNI, like [cilium](https://github.com/cilium/cilium), [calico](https://github.com/projectcalico/calico), they have good implement of IPAM, so, the Spiderpool is not intentionally designed to these cases, but maybe integrate with them.

The Spiderpool is intentionally designed to use for underlay network, and administrator could accurately manage each IP.

Currently, in the community, the IPAM plugin, such as [whereabout](https://github.com/k8snetworkplumbingwg/whereabouts), [kube-ipam](https://github.com/cloudnativer/kube-ipam),
[static](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/static),
[dhcp](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/dhcp), [host-local](https://github.com/containernetworking/plugins/tree/main/plugins/ipam/host-local).
, few of them could help solve complex underlay-network needs, so, we decide to develop the spiderpool.

BTW, there is also some CNI plugins who could work on underlay mode, such as [kube-ovn](https://github.com/kubeovn/kube-ovn) and [coil](https://github.com/cybozu-go/coil).
But the spiderpool provides lots of different features, you could refer Feature for detail

## Feature

* Based on CRD storage. All operation could be done with kubernetes API-server

* Support assigning IP Address of IPv4-only, IPv6-only and dual-stack

* Support working on the cluster of IPv4-only, IPv6-only and dual-stack

* Support creating multiple ippool. Different namespace and application could monopolize or share an ippool

* An application could specify multiple backup ippool resource, in case that IP in an ippool is out of use. Therefore, no need to scale up the IP resource in a fixed ippool, or no need to modify the application yaml to change a ippool

* Support assign fixed IP for application. No need to hard code the IP list of pod yaml, which is not easy to modify it, just set the selector field of ippool and scale up or down the ippool.

* Support assign IP by order for statefulset pod

* For an application who deployed in different subent or zone, different pods in a same controller could get IP from different subnet.

* Administrator could safely edit ippool resource, the spiderpool will help validate the modification and prevent from data race.

* real-time resource collection, especially for solving IP leaking or slow collection, which may make new pod fail to assign IP.

* Support ranges of CNI (Container Network Interface) plugin, who support third-party IPAM plugin. Especially, the Spiderpool could help much for CNI like [spiderflat](https://github.com/spidernet-io/spiderflat),
  [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
  [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
  [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
  [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
  [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni)

* Especially support [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) case to assign IP for multiple interfaces.

* good performance for assigning and collect ip

* support global and ippool-range reserve IP, who will not assign even included in an ippool

* reach metrics for look into IP usage and issue

* support amd64 and arm64

## Components

refer to [architecture](docs/concepts/arch.md) for components

## Installation

refer to [installation](./docs/usage/install.md)

## Usage

for quick start, refer to [demo](./docs/usage/demo.md)

for complex case, refer to [ippool usage](./docs/usage/allocation.md)

## Development

[Development guide](docs/contributing/pullrequest.md) is a reference point for development helper commands

## Roadmap

refer to [roadmap](docs/concepts/roadmap.md)

## License

Spider pool is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for the full license text.
