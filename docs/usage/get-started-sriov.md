# SRIOV Quick Start

**English** | [**简体中文**](./get-started-sriov-zh_CN.md)

Spiderpool provides a solution for assigning static IP addresses in underlay networks. In this page, we'll demonstrate how to build a complete underlay network solution using [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Macvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan), [Veth](https://github.com/spidernet-io/plugins), and [Spiderpool](https://github.com/spidernet-io/spiderpool), which meets the following kinds of requirements:

* Applications can be assigned static Underlay IP addresses through simple operations.

* Pods with multiple Underlay NICs connect to multiple Underlay subnets.

* Pods can communicate in various ways, such as Pod IP, clusterIP, and nodePort.

TODO
