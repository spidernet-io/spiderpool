# 安装

[**English**](./install.md) | **简体中文**

## 用法

安装 Spiderpool 有两种场景：

- 在 Underlay NICs 下安装 Spiderpool

    对于这一使用场景，集群可以使用一个或多个 Underlay CNI 来运行 Pod。

    当 Pod 中有一个或多个 Underlay CNI 时，Spiderpool 可以帮助其分配 IP 地址、调整路由、连接 Pod 和本地节点、检测 IP 冲突等。

- 为 Overlay CNI 的 Pod 添加 Underaly CNI 的辅助网卡

    对于这一使用场景，集群可以使用一个 Overlay CNI 和其他 Underlay CNI 来运行 Pod。

    当一个 Pod 中有一个或多个不同的网卡时，Spiderpool 可以帮助分配 IP 地址、调整路由、连接 Pod 和本地节点、检测 IP 冲突等。

## 在 Underlay NICs 下安装 Spiderpool

任何与第三方 IPAM 插件兼容的 CNI 项目都可以与 Spiderpool 良好配合，例如：

[macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
[sriov CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
[ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni),
[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni),
[calico CNI](https://github.com/projectcalico/calico),
[weave CNI](https://github.com/weaveworks/weave)

以下是 Underlay NICs 下安装 Spiderpool 的示例：

- [macvlan in Kind](./underlay/get-started-kind-zh_CN.md)

- [SRIOV CNI](./underlay/get-started-sriov-zh_CN.md), 适合裸机主机这样的场景。

- [calico CNI](./underlay/get-started-calico-zh_CN.md)

- [weave CNI](./underlay/get-started-weave-zh_CN.md)

- [ovs CNI](./underlay/get-started-ovs-zh_CN.md), 适合裸机主机这样的场景。

以下示例是在集群中使用两个 CNI 的高级示例：

- [SRIOV and macvlan](./underlay/get-started-macvlan-and-sriov.md)，这个适用于裸机主机等场景，有些节点有 SRIOV 网卡而有些节点没有

## 在云基础设施上安装 Underlay CNI

- [alibaba cloud](./cloud/get-started-alibaba.md)

- [vmware vsphere](./cloud/get-started-vmware.md)

- [openstack](./cloud/get-started-openstack.md)

## 为 Overlay CNI 的 Pod 添加 Underaly CNI 的辅助网卡

以下示例是安装 Spiderpool 的指南：

- [calico and macvlan CNI](./overlay/get-started-calico.md)

- [cilium and macvlan CNI](./overlay/get-started-cilium.md)

## 卸载

一般情况下，您可以通过以下方式卸载当前的 Spiderpool 版本：

```bash
helm uninstall spiderpool -n kube-system
```

然而，Spiderpool 的某些 CR 中存在 [finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/)，`helm uninstall` cmd 可能无法清理所有相关的 CR。 获取下列示例的清理脚本并执行它，以确保下次部署 Spiderpool 时不会出现意外错误。

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/scripts/cleanCRD.sh
chmod +x cleanCRD.sh && ./cleanCRD.sh
```
