# 使用索引

[**English**](./readme.md) | **简体中文**

## 安装 Spiderpool

### 在裸金属环境上安装 Spiderpool

集群网络可以为 Pod 接入 Spiderpool 一个或多个 Underlay 网络的网卡，从而让 Pod 具备接入 underlay 网络的能力，具体可参考 [一个或多个 underlay CNI 协同](../concepts/arch-zh_CN.md#应用场景pod-接入若干个-underlay-cni-网卡)

以下是安装示例：

- [创建集群：基于 macvlan 网络的集群](./install/underlay/get-started-macvlan-zh_CN.md)

- [创建集群：基于 SR-IOV CNI 网络的集群](./install/underlay/get-started-sriov-zh_CN.md)

- [创建集群：基于 ovs 网络的集群](./install/underlay/get-started-ovs-zh_CN.md)

- [创建集群：基于 calico CNI 提供固定 IP 的集群](./install/underlay/get-started-calico-zh_CN.md)

- [创建集群：基于 weave CNI 提供固定 IP 的集群](./install/underlay/get-started-weave-zh_CN.md)

- [创建集群：基于 underlay CNI 为应用提供 RDMA RoCE 通信](./rdma-roce-zh_CN.md)

- [创建集群：基于 underlay CNI 为应用提供 RDMA Infiniband 通信](./rdma-ib-zh_CN.md)

### 在虚拟机和公有云环境上安装 Spiderpool

在公有云和虚拟机环境上运行 Spiderpool，使得 POD 对接 VPC 的网络

- [创建集群：在阿里云上基于 ipvlan 的网络](./install/cloud/get-started-alibaba-zh_CN.md)

- [创建集群：VMware vsphere](./install/cloud/get-started-vmware-zh_CN.md)

- [创建集群：在 AWS 基于 ipvlan 的网络](./install/cloud/get-started-aws-zh_CN.md)

- 在 VMware vSphere 平台上运行 ipvlan CNI，无需打开 vSwitch 的["混杂"转发模式](https://docs.vmware.com/cn/VMware-vSphere/8.0/vsphere-security/GUID-3507432E-AFEA-4B6B-B404-17A020575358.html) ，从而确保 vSphere 平台的转发性能。参考[安装](./install/cloud/get-started-vmware-zh_CN.md)

### 基于 Overlay CNI 和 Spiderpool 的双 CNI 集群

集群网络可以为 Pod 同时接入一个 Overlay CNI 网卡和多个 spiderpool 的 Underlay CNI 辅助网卡，从而让 Pod 同时具备接入 overlay 和 underlay 网络的能力，具体可参考 [underlay CNI 和 overlay CNI 协同](../concepts/arch-zh_CN.md#应用场景pod-接入一个-overlay-cni-和若干个-underlay-cni-网卡) 。以下是安装示例：

- [创建集群：基于 kind 集群的双网络](./install/get-started-kind-zh_CN.md)

- [创建集群：基于 calico 和 macvlan CNI 的双网络](./install/overlay/get-started-calico-zh_cn.md)

- [创建集群：基于 Cilium 和 macvlan CNI 的双网络](./install/overlay/get-started-cilium-zh_cn.md)

### TLS 证书

安装 spiderpool 时，可指定 TLS 证书的生成方式，可参考 [文章](./install/certificate.md)

## 卸载 Spiderpool

可参考 [卸载](./install/uninstall-zh_CN.md)

## 升级 Spiderpool

可参考 [升级](./install/upgrade-zh_CN.md)

## 使用 Spiderpool

### IPAM 功能

- 应用可以共享一个 IP 池，可参考[例子](./spider-affinity-zh_CN.md)。

- 对于无状态应用，可以独享一个 IP 地址池，并固定所有 Pod 的 IP 使用范围。 可参考[例子](./spider-subnet-zh_CN.md)。

- 对于有状态应用，支持为每一个 Pod 持久化分配固定 IP 地址，同时在扩缩时可控制所有 Pod 所使用的 IP 范围，可参考[例子](./statefulset-zh_CN.md)。

- 支持为 kubevirt 提供 underlay 网络，固定虚拟机的 IP 地址，可参考 [例子](./kubevirt-zh_CN.md)

- 对于一个跨子网部署的应用，支持为其不同副本分配不同子网的 IP 地址，可参考[例子](./network-topology-zh_CN.md)。

- Subnet 功能，一方面，能够实现基础设施管理员和应用管理员的职责分离，
  另一方面，能够为有固定 IP 需求的应用自动管理 IP 池，包括自动创建、扩缩容 IP、删除 固定 IP 池，
  这能够减少大量的运维负担，可参考[例子](./spider-subnet-zh_CN.md)。

  该功能除了支持 K8S 原生的应用控制器，同时支持基于 operator 实现的第三方应用控制器。
  可参考[例子](./operator-zh_CN.md)。

- 可以设置集群级别的默认 IP 池，也可租户级别的默认 IP 池。同时，IP 池既可以被整个集群共享，
  也可被限定为被一个租户使用。可参考[例子](./spider-affinity-zh_CN.md)。

- 基于节点拓扑的 IP 池功能，满足每个节点精细化的子网规划需求，可参考[例子](./network-topology-zh_CN.md)

- 可以通过 IP 池和 Pod annotaiton 等多种方式定制自定义路由，可参考[例子](./route-zh_CN.md)。

- 应用可设置多个 IP 池，实现 IP 资源的备用效果。可参考[例子](./spider-ippool-zh_CN.md)。

- 设置全局的预留 IP，让 IPAM 不分配出这些 IP 地址，这样能避免与集群外部的已用 IP 冲突。
  可参考[例子](./reserved-ip-zh_CN.md)。

- 分配和释放 IP 地址的高效性能，可参考[报告](../concepts/ipam-performance-zh_CN.md)。

- 合理的 IP 回收机制设计，使得集群或应用在故障恢复过程中，能够及时分配到 IP 地址。可参考[例子](../concepts/ipam-des-zh_CN.md)。

### 多网卡功能

- 支持为 Pod 多网卡分配不同子网的 IP 地址；帮助所有网卡之间协调策略路由，以确保请求向和回复向数据路径一致，避免丢包；支持定制哪张网卡的网关作为缺省路由。

  对于 Pod 具备多个 underlay CNI 网卡场景，可参考[例子](./multi-interfaces-annotation.md)。

  对于 Pod 具备一个 overlay 网卡和多个 underlay CNI 网卡场景，可参考[例子](./install/overlay/get-started-calico-zh_cn.md)。

### 连通性功能

- 支持 RDMA 网卡的 shared 和 exclusive 模式，能基于 maclan、ipvlan 和 SR-IOV CNI 为应用提供 RDMA 通信设备。具体可参考 [Roce 例子](./rdma-roce-zh_CN.md) 和 [IB 例子](./rdma-ib-zh_CN.md).

- coordinator 插件能够依据网卡的 IP 地址来重新配置 MAC 地址，使两者一一对应，从而能够有效避免网络中的交换路由设备更新 ARP 转发规则，避免丢包。可参考 [文章](../concepts/coordinator-zh_CN.md#支持固定-pod-的-mac-地址前缀)。

- 对 [Macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
  [vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
  [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
  [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
  [ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni) 等，
  提供了基于 kube-proxy 和 eBPF kube-proxy replacement 访问 ClusterIP 访问，并联通 Pod 和宿主机通信，解决 Pod 健康检查问题。
  可参考[例子](./underlay_cni_service-zh_CN.md)。

- 能够帮助实施 IP 地址冲突检测、网关可达性检测，以保证 Pod 通信正常，可参考[例子](../concepts/coordinator.md)。

- 多集群网络可基于相同的 underlay 网络或者 [Submariner](./submariner-zh_CN.md) 实现联通。

### 运维管理功能

- 在 Pod 启动时，能够在宿主机上动态创建 BOND 接口和 VLAN 子接口，以帮助
  [Macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan)
  和 [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan)
  准备 master 接口。可参考[例子](../reference/plugin-ifacer.md)。

- 以最佳实践的 CNI 配置来便捷地生成 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)
  NetworkAttachmentDefinition 实例，并且保证其正确的 JSON 格式来提高使用体验。
  可参考[例子](./spider-multus-config-zh_CN.md)。

### 其它功能

- [指标](../reference/metrics.md)

- 支持 AMD64 和 ARM64

- 所有的功能都能够在 ipv4-only、ipv6-only、dual-stack 场景下工作。可参考[例子](./spider-ippool-zh_CN.md)。
