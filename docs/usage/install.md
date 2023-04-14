# Installation

## Install

Any CNI project compatible with third-party IPAM plugins, can work well with spiderpool, such as:

* [macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan)

* [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan)

* [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan)

* [sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni)

* [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni)

* [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni)

* [calico CNI](https://github.com/projectcalico/calico)

* [weave CNI](https://github.com/weaveworks/weave)

The following examples are guides to install spiderpool with different CNI:

* [spiderpool with macvlan in Kind](./get-started-kind.md)

* [spiderpool with macvlan CNI](./get-started-macvlan.md)

* [spiderpool with SRIOV CNI](./get-started-sriov.md)

* [spiderpool with calico CNI](./get-started-calico.md)

* [spiderpool with weave CNI](./get-started-weave.md)

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
