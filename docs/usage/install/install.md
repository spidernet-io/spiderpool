# Installation

**English** | [**简体中文**](./install-zh_CN.md)

## usage

It could have two kinds of scenes to install spiderpool:

* spiderpool for underlay NICs

    For this use case, the cluster could use one or more underlay CNI to run pods.

    When one or more underlay NIC in a pod, spiderpool could help assign IP address, tune routes, connect the pod and local node, detect IP conflict etc.

* spiderpool for overlay and underlay NICs

    For this use case, the cluster could use one overlay CNI and other underlay CNI to run pods.

    When one or more NIC of different NIC in a pod, spiderpool could help assign IP address, tune routes, connect the pod and local node, detect IP conflict etc.

## Install Spiderpool in Underlay NICs

Any CNI project compatible with third-party IPAM plugins, can work well with spiderpool, such as:
[macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
[sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
[ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni),
[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni),
[calico CNI](https://github.com/projectcalico/calico),
[weave CNI](https://github.com/weaveworks/weave)

The following examples are guides to install spiderpool:

* [macvlan in Kind](./underlay/get-started-kind.md)

* [SRIOV CNI](./underlay/get-started-sriov.md), this is suitable for scenes like bare metal host.

* [calico CNI](./underlay/get-started-calico.md)

* [weave CNI](./underlay/get-started-weave.md)

* [ovs CNI](./underlay/get-started-ovs.md), this is suitable for scenes like bare metal host.

The following examples are advanced to use two CNI in a cluster:

* [SRIOV and macvlan](./underlay/get-started-macvlan-and-sriov.md), this is suitable for scenes like bare metal hosts, some nodes has SRIOV NIC and some nodes do not have

## Installation for underlay CNI on Cloud infrastruct

* [alibaba cloud](./cloud/get-started-alibaba.md)

* [vmware vsphere](./cloud/get-started-vmware.md)

* [openstack](./cloud/get-started-openstack.md)

## Installation for adding an auxiliary underlay CNI NIC for overlay CNI

The following examples are guides to install spiderpool:

* [calico and macvlan CNI](./overlay/get-started-calico.md)

* [cilium and macvlan CNI](./overlay/get-started-cilium.md)

## Uninstall

Generally, you can uninstall Spiderpool release in this way:

```bash
helm uninstall spiderpool -n kube-system
```

However, there are [finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) in some CRs of Spiderpool, the `helm uninstall` cmd may not clean up all relevant CRs. Get this cleanup script and execute it to ensure that unexpected errors will not occur when deploying Spiderpool next time.

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/scripts/cleanCRD.sh
chmod +x cleanCRD.sh && ./cleanCRD.sh
```
