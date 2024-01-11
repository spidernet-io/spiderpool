# Coordinator

**简体中文** | [**English**](coordinator.md) 

Spiderpool 内置一个叫 `coordinator` 的 CNI meta-plugin, 它在 Main CNI 被调用之后再工作，它主要提供以下几个主要功能:

- 解决 underlay Pod 无法访问 ClusterIP 的问题
- 在 Pod 多网卡时，调谐 Pod 的路由，确保数据包来回路径一致
- 支持检测 Pod 的 IP 是否冲突
- 支持检测 Pod 的网关是否可达
- 支持固定 Pod 的 Mac 地址前缀

注意: 如果您的操作系统是使用 NetworkManager 的 OS，比如 Fedora、Centos等，强烈建议配置 NetworkManager 的配置文件(/etc/NetworkManager/conf.d/spidernet.conf)，避免 NetworkManager 干扰 `coordinator` 创建的 Veth 虚拟接口，影响通信:

```shell
~# cat << EOF | > /etc/NetworkManager/conf.d/spidernet.conf
> [keyfile]
> unmanaged-devices=interface-name:^veth*
> EOF
~# systemctl restart NetworkManager
```

下面我们将详细的介绍 `coordinator` 如何解决或实现这些功能。

## CNI 配置字段说明

| Field              | Description                                                                                                                                                                                                                                                             | Schema   | Validation | Default                           |
|--------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|------------|-----------------------------------|
| type               | CNI 的类型                                                                                                                                                                                                                                                                 | 字符串      | required   | coordinator                       |
| mode               | coordinator 运行的模式. "auto": coordinator 自动判断运行在 Underlay 或者 Overlay; "underlay": 为 Pod 创建一对 Veth 设备，用于转发集群东西向流量。由 Pod 的 Underlay 网卡转发南北向流量; "overlay": 不额外创建 veth 设备，运行在多网卡模式。由 overlay 类型的 CNI(calico，cilium) 转发集群东西向流量，由 underlay 网卡转发南北向流量; "disable": 禁用 coordinator | 字符串      | optional   | auto                              |
| tunePodRoutes      | Pod 多网卡模式下，是否调协 Pod 的路由，解决访问来回路径不一致的问题                                                                                                                                                                                                                                  | 布尔型      | optional   | true                              |
| podDefaultRouteNic | Pod 多网卡时，配置 Pod 的默认路由网卡。默认为 "", 其 value 实际为 Pod 第一张拥有默认路由的网卡                                                                                                                                                                                                            | 字符串      | optional   | ""                                |
| podDefaultCniNic   | K8s 中 Pod 默认的第一张网卡                                                                                                                                                                                                                                                      | 布尔型      | optional   | eth0                              |
| detectGateway      | 创建 Pod 时是否检查网关是否可达                                                                                                                                                                                                                                                      | 布尔型      | optional   | false                             |
| detectIPConflict   | 创建 Pod 时是否检查 Pod 的 IP 是否冲突                                                                                                                                                                                                                                              | 布尔型      | optional   | false                             |
| podMACPrefix       | 是否固定 Pod 的 Mac 地址前缀                                                                                                                                                                                                                                                     | 字符串      | optional   | ""                                |
| overlayPodCIDR     | 默认的集群 Pod 的子网，会注入到 Pod 中。不需要配置，自动从 Spidercoordinator default 中获取                                                                                                                                                                                                        | []stirng | optional   | 默认从 Spidercoordinator default 中获取 |
| serviceCIDR        | 默认的集群 Service 子网， 会注入到 Pod 中。不需要配置，自动从 Spidercoordinator default 中获取                                                                                                                                                                                                    | []stirng | optional   | 默认从 Spidercoordinator default 中获取 |
| hijackCIDR         | 额外的需要从主机转发的子网路由。比如nodelocaldns 的地址: 169.254.20.10/32                                                                                                                                                                                                                    | []stirng | optional   | 空                                 |
| hostRuleTable      | 策略路由表号，同主机与 Pod 通信的路由将会存放于这个表号                                                                                                                                                                                                                                          | 整数型      | optional   | 500                               |
| hostRPFilter       | 设置主机上的 sysctl 参数 rp_filter                                                                                                                                                                                                                                              | 整数型      | optional   | 0                                 |
| txQueueLen         | 设置 Pod 的网卡传输队列                                                                                                                                                                                                                                                          | 整数型      | optional   | 0                                 |
| detectOptions      | 检测地址冲突和网关可达性的高级配置项: 包括重试次数(默认为 3 次), 探测间隔(默认为 1s) 和 超时时间(默认为 1s)                                                                                                                                                                                                        | 对象类型     | optional   | 空                                 |
| logOptions         | 日志配置，包括 logLevel(默认为 debug) 和 logFile(默认为 /var/log/spidernet/coordinator.log)                                                                                                                                                                                           | 对象类型     | optional   | -                                 |

> 如果您通过 `SpinderMultusConfig CR`  帮助创建 NetworkAttachmentDefinition CR，您可以在 `SpinderMultusConfig` 中配置 `coordinator` (所有字段)。参考: [SpinderMultusConfig](../reference/crd-spidermultusconfig.md)。
>
> `Spidercoordinators CR` 作为 `coordinator` 插件的全局缺省配置(所有字段)，其优先级低于 NetworkAttachmentDefinition CR 中的配置。 如果在 NetworkAttachmentDefinition CR 未配置, 将使用 `Spidercoordinator CR` 作为缺省值。更多详情参考: [Spidercoordinator](../reference/crd-spidercoordinator.md)。

## 解决 underlay Pod 无法访问 ClusterIP 的问题

我们在使用一些如 Macvlan、IPvlan、SR-IOV 等 Underlay CNI时，会遇到其 Pod 无法访问 ClusterIP 的问题，这常常是因为 underlay Pod 访问 CLusterIP 需要经过在交换机的网关，但网关上并没有去往
ClusterIP 的路由，导致无法访问。

关于 Underlay Pod 无法访问 ClusterIP 的问题，请参考 [Underlay-CNI访问 Service](../usage/underlay_cni_service-zh_CN.md)

## 支持检测 Pod 的 IP 是否冲突( alpha 阶段)

对于 Underlay 网络，IP 冲突是无法接受的，这可能会造成严重的问题。在创建 Pod 时，我们可借助 `coordinator` 检测 Pod 的 IP 是否冲突，支持同时检测 IPv4 和 IPv6 地址。通过发送 ARP 或 NDP 探测报文，
如果发现回复报文的 Mac 地址不是来自 Pod 本身的网卡，那我们认为这个 IP 是冲突的，并拒绝 IP 冲突的 Pod 被创建。
此外，我们默认还会对发生 IP 冲突的**无状态**的 Pod 释放所有的已分配的 IP 使其重新分配，使得 Pod 在下一次重新调用 CNI 时能够尝试分配到其它非冲突的 IP。对于发生 IP 冲突的**有状态**的 Pod，为了保持 IP 地址也是有状态设计，我们不会对其释放。您可通过 spiderpool-agent [环境变量](../reference/spiderpool-agent.md#env) `SPIDERPOOL_ENABLED_RELEASE_CONFLICT_IPS` 来控制此功能。

我们可以通过 Spidermultusconfig 配置它:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: detect-ip
  namespace: default
spec:
  cniType: macvlan
  macvlan:
    master: ["eth0"]
  coordinator:
    detectIPConflict: true    # Enable detectIPConflict
```

> 若 IP 冲突检查发现某 IP 已被占用，请检查是否被集群中其他处于 `Terminating` 阶段的 **无状态** Pod 所占用，并配合 [IP 回收机制](./ipam-des-zh_CN.md#ip-回收机制) 相关参数进行配置。

## 支持检测 Pod 的网关是否可达(alpha)

在 Underlay 网络下，Pod 访问外部需要通过网关转发。如果网关不可达，那么在外界看来，这个 Pod 实际是失联的。有时候我们希望创建 Pod 时，其网关是可达的。 我们可借助 `coordinator` 检测 Pod 的网关是否可达，
支持检测 IPv4 和 IPv6 的网关地址。我们通过发送 ICMP 报文，探测网关地址是否可达。如果网关不可达，将会阻止 Pod 创建:

我们可以通过 Spidermultusconfig 配置它:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: detect-gateway
  namespace: default
spec:
  cniType: macvlan
  macvlan:
    master: ["eth0"]
  enableCoordinator: true
  coordinator:
    detectGateway: true    # Enable detectGateway
```

> 注意: 有一些交换机不允许被 arp 探测，否则会发出告警，在这种情况下，我们需要设置 detectGateway 为 false

## 支持固定 Pod 的 Mac 地址前缀(alpha)

有一些传统应用可能需要通过固定的 Mac 地址或者 IP 地址来耦合应用的行为。比如 License Server 可能需要应用固定的 Mac 地址或 IP 地址为应用颁发 License。如果 Pod 的 Mac 地址发生改变，已颁发的 License 可能无效。
所以需要固定 Pod 的 Mac 地址。 Spiderpool 可通过 `coordinator` 固定应用的 Mac 地址，固定的规则是配置 Mac 地址前缀(2字节) + 转化 Pod 的 IP(4字节) 组成。

注意:

> 目前支持修改 Macvlan 和 SR-IOV 作为 CNI 的 Pod。 IPVlan L2 模式下主接口与子接口 Mac 地址一致，不支持修改
>
> 固定的规则是配置 Mac 地址前缀(2字节) + 转化 Pod 的 IP(4字节) 组成。一个 IPv4 地址长度 4 字节，可以完全转换为2 个 16 进制数。对于 IPv6 地址，只取最后 4 个字节。

我们可以通过 Spidermultusconfig 配置它:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: overwrite-mac
  namespace: default
spec:
  cniType: macvlan
  macvlan:
    master: ["eth0"]
  enableCoordinator: true
  coordinator:
    podMACPrefix: "0a:1b"    # Enable detectGateway
```

当 Pod 创建完成，我们可以检测 Pod 的 Mac 地址的前缀是否是 "0a:1b"

## 配置网卡传输队列(txQueueLen)

传输队列长度（txqueuelen）是TCP/IP协议栈网络接口的一个值，它设置了网络接口设备内核传输队列中允许的数据包数量。如果txqueuelen值过小，可能导致在Pod之间的通信中丢失数据包。如果需要，我们可以对其进行配置:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: txqueue-demo 
  namespace: default
spec:
  cniType: macvlan
  macvlan:
    master: ["eth0"]
  enableCoordinator: true
  coordinator:
    txQueueLen: 2000 
```

## 自动获取集群 Service 的 CIDR

Kubernetes 1.29 开始支持以 ServiceCIDR 资源的方式配置集群 Service 的 CIDR，更多信息参考 [KEP 1880](https://github.com/kubernetes/enhancements/blob/master/keps/sig-network/1880-multiple-service-cidrs/README.md)。如果您的集群支持 ServiceCIDR，Spiderpool-controller 组件 自动监听 ServiceCIDR 资源的变化，将读取到的 Service 子网信息自动更新到 Spidercoordinator 的 Status 中。

```shell
~# kubectl get servicecidr kubernetes -o yaml
apiVersion: networking.k8s.io/v1alpha1
kind: ServiceCIDR
metadata:
  creationTimestamp: "2024-01-25T08:36:00Z"
  finalizers:
  - networking.k8s.io/service-cidr-finalizer
  name: kubernetes
  resourceVersion: "504422"
  uid: 72461b7d-fddd-409d-bdf2-83d1a2c067ca
spec:
  cidrs:
  - 10.233.0.0/18
  - fd00:10:233::/116
status:
  conditions:
  - lastTransitionTime: "2024-01-28T06:38:55Z"
    message: Kubernetes Service CIDR is ready
    reason: ""
    status: "True"
    type: Ready

~# kubectl get spidercoordinators.spiderpool.spidernet.io default -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderCoordinator
metadata:
  creationTimestamp: "2024-01-25T08:41:50Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 1
  name: default
  resourceVersion: "41645"
  uid: d1e095db-d6e8-4413-b60e-fcf31ad2bf5e
spec:
  detectGateway: false
  detectIPConflict: false
  hijackCIDR:
  - 10.244.64.0/18
  - fd00:10:244::/112
  hostRPFilter: 0
  hostRuleTable: 500
  mode: auto
  podCIDRType: auto
  podDefaultRouteNIC: ""
  podMACPrefix: ""
  tunePodRoutes: true
  txQueueLen: 0
status:
  phase: Synced
  serviceCIDR:
  - 10.233.0.0/18
  - fd00:10:233::/116
```

## 已知问题

- underlay 模式下，underlay Pod 与 Overlay Pod(calico or cilium) 进行 TCP 通信失败

  此问题是因为数据包来回路径不一致导致，发出的请求报文匹配源Pod 侧的路由，会通过 `veth0` 转发到主机侧，再由主机侧转发至目标 Pod。 目标 Pod 看见数据包的源 IP 为 源 Pod 的 Underlay IP，直接走 Underlay 网络而不会经过源 Pod 所在主机。
  在该主机看来这是一个非法的数据包(意外的收到 TCP 的第二次握手报文，认为是 conntrack table invalid), 所以被 kube-proxy 的一条 iptables 规则显式的 drop 。 目前可以通过切换 kube-proxy 的模式为 ipvs 规避。这个问题预计在 k8s 1.29 修复。
  当 sysctl `nf_conntrack_tcp_be_liberal` 设置为 1 时，kube-proxy 将不会下发这条 DROP 规则。

- overlay 模式下, 当 Pod 附加多张网卡时。如果集群的缺省CNI 为 Cilium, Pod 的 underlay 网卡 无法与节点通信。

  我们借助缺省CNI创建 Veth 设备，实现 Pod 的 underlay IP 与节点通信(正常情况下，macvlan 在 bridge 模式下， 其父子接口无法直接)，但 Cilium 不允许非 Cilium 子网的 IP 从 Veth 设备转发。
