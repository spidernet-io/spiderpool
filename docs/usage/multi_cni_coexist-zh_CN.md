# 多 CNI 共存于一个集群

**简体中文** | [**English**](./multi_cni_coexist.md)

## 背景

CNI 作为 Kubernetes 的集群中重要的组件。一般情况下，都会部署一个 CNI(比如 Calico)，由它负责集群网络的连通性。有些时候一些终端用户基于性能、安全等的考虑，会在集群中使用多种类型的 CNI，比如 Underlay 类型的 Macvlan CNI。此时一个集群就可能存在多种 CNI 类型的 Pod，不同类型的 Pod 分别适用于不同的场景：

* 单 Calico 网卡的 Pod: 如 CoreDNS 等系统组件，没有固定 IP 的需求，也不需要南北向流量通信，只存在集群东西向流量通信的需求。
* 单 Macvlan 网卡的 Pod: 适用于对性能，安全有特殊要求的应用，或需要以 Pod IP 直接南北向流量通信的传统上云应用。
* Calico 和 Macvlan 网卡的多网卡 Pod：同时兼顾上面二者的需求。既需要以固定的 Pod IP 访问集群南北向流量，又需要访问集群东西向流量(比如和 Calico Pod 或 Service)。

另外，当多 CNI 的 Pod 存在于一个集群，实际上这个集群存在两种不同的数据转发方案: Underlay 和 Overlay。这可能会导致一些其他问题:

* 使用 Underlay 网络的 Pod 无法与集群中使用 Overlay 网络的 Pod 直接通信: 由于转发路径不一致，Overlay 网络常常需要经过节点作二次转发，但 Underlay 一般直接通过底层网关转发。所以当二者互相访问时，可能由于底层交换机未同步集群子网的路由导致丢包
* 双 CNI 可能会增加使用和运维复杂度，比如 IP 地址管理等

Spiderpool 这一套完整的 Underlay 网络解决方案可以解决当集群存在多种 CNI 时的互联互通问题，又可以减轻 IP 地址运维负担。下面我们将介绍它们之间的数据转发流程。

## 快速开始

* Calico + Macvlan 多网卡快速开始可参考 [get-stared-calico](./install/overlay/get-started-calico-zh_cn.md)
* 单 Macvlan 网卡可参考 [get-started-macvlan](./install/underlay/get-started-macvlan-zh_CN.md)
* Underlay CNI 访问 Service 可参考 [underlay_cni_service](./underlay_cni_service-zh_CN.md)

## 数据转发流程

![dataplane](../images/underlay_overlay_cni.png)

下面介绍几种典型的通信场景:

### Calico Pod 访问 Calico 和 Macvlan 多网卡的 Pod IP

如[数据转发流程图](#数据转发流程)中标注的 `1` 和 `2` 的线路所示:

1. 请求数据包按照 `1` 的线路，从 Pod1(单 calico 网卡 pod) 经过其 calixxx 虚拟网卡转发到节点 node1 的网络协议栈上，经过节点 node1 和 node2 之间的路由，转发到目标主机 node 2。

2. 无论访问的是目标 Pod2(Calico 和 Macvlan 多网卡) 的 Calico 网卡(10.233.100.2)还是 Macvlan 网卡 IP(10.7.200.1)，都会经过 Pod2 (Calico 和 Macvlan 多网卡 Pod) 对应的 calixxx 虚拟网卡转发到 Pod 中。

    由于 Macvlan bridge 模式的限制，master 父子接口之间无法直接通信，所以导致节点无法直接访问 pod 的 macvlan IP， spiderpool 会在节点为 Pod 的 macvlan 网卡注入一条通过 calixxx 转发 macvlan 父子接口通信的路由。

3. Pod2(Calico 和 Macvlan 多网卡 Pod) 发起回复报文时，按照 `2` 的线路: 由于目标 Pod1 的 IP 为: 10.233.100.1，命中 Pod2 中设置的 Calico 子网路由(如下)，这样所有访问 calico 子网目标会从 eth0 转发到节点 node2 的网络协议栈。

        ～# kubectl  exec -it calico-macvlan-556dddfdb-4ctxv -- ip r
        10.233.64.0/18 via 10.7.168.71 dev eth0 src 10.233.100.2

4. 由于回复报文的目标 IP 为 Pod1(单 calico 网卡 IP): 10.233.100.1，所以会匹配 calico 子网的隧道路由再转发到目标节点 node1。最后通过 Pod1 对应的 calixxx 虚拟网卡，转发到 pod1，整个访问结束。

### Calico+Macvlan 多网卡的 Pod 访问 Calico Pod 的 Service

如[数据转发流程图](#数据转发流程)中标注的 `1` 和 `2` 的线路所示:

1. 如图中所示的 pod1（单calico网卡pod）和 pod2（具备calico 和 macvlan网卡的pod）都使用了 calico ip 返回给 kubelet 作为 PodIP，因此，他们直接进行常规通信时，都是以对方的 calico ip为目标来发起访问。

2. 当 pod2 主动访问 pod1 的 clusterip 时，由于 spiderpool 在 pod 设置的路由：访问 Service 的数据包都以 calico 网卡的 IP 作为源地址，并从 eth0 转发到节点 node2 上。如下 10.233.0.0/18 是 service的子网：

        ～# kubectl  exec -it calico-macvlan-556dddfdb-4ctxv -- ip r
        10.233.0.0/18 via 10.7.168.71 dev eth0 src 10.233.100.2

3. 经过节点 node2 网络协议栈上的 kube-proxy 解析其目标 clusterip 地址为 pod1 (单 calico 网卡的 pod）: 10.233.100.1, 随后通过 calico 设置的节点隧道路由转发到目标主机 node1，最后通过 pod1 对应的 calixxx 虚拟网卡，转发到 Pod1。

4. Pod1 发起的回复数据包按照线路 `1` ，通过 calixxx 虚拟网卡转发到节点 node1 上，随后通过主机之间的隧道路由转发到节点 node2, 随后 node2 的 kube-proxy 将源地址改为 clusterip 的地址，随后通过 calixxx 虚拟网卡发送到 Pod2 中。整个访问结束。

### Macvlan Pod 访问 Calico Pod

如[数据转发流程图](#数据转发流程)中标注的 `3`，`4` 和 `6` 的线路所示:

1. Spiderpool 会在 Pod 内部注入通往 calico 子网通过 veth0 转发的路由表项。如下 10.233.64.0/18 是 calico 子网，该路由确保 Pod3(单 Macvlan 网卡 pod) 访问 Pod1 (单 calico 网卡 pod)时，将按照线路 `3`  通过 veth0 转发到节点 node3。

        ~# kubectl exec -it macvlan-76c49c7bfb-82fnt -- ip r
        10.233.64.0/18 via 10.7.168.71 dev veth0 src 10.7.200.2

2. 转发到节点 node3 之后，由于目标 pod1 的 IP 是 10.233.100.1，所以数据包通过 calico 的隧道路由转发到节点 node 1上，再通过 Pod1 对应的 calixxx 虚拟网卡转发到 pod1。
3. 但 pod1 (单 calico 网卡 pod) 在按照线路 `4` 将回复报文发送到节点 node1，由于此时的目标 pod3(单 macvlan pod) 的 IP 为 10.7.200.2，于是按照线路 `6` 直接将数据包转发到 pod3，而不会经过节点转发，导致了数据包来回转发路径不一致，可能会被内核认为其数据包的 conntrack 的 state 为 invalid，会被 kube-proxy 的一条 iptables 规则丢弃:

        ~# iptables-save  -t filter | grep '--ctstate INVALID -j DROP'
        iptables -A FORWARD -m conntrack --ctstate INVALID -j DROP

        该规则原是为了解决 [#Issue 74839](https://github.com/kubernetes/kubernetes/issues/74839) 提出的问题，因为 某些 tcp 报文大小超出窗口限制，导致被内核标记其 conntrack 的 state 为 invalid，从而导致整个 tcp 链接被 reset。于是 k8s 社区通过下发这条规则来解决这个问题，但这条规则可能会影响此场景中数据包来回不一致的情况。如社区有相关的 issue 报告：[#Issue 117924](https://github.com/kubernetes/kubernetes/issues/117924), [#Issue 94861](https://github.com/kubernetes/kubernetes/issues/94861),[#Issue 177](https://github.com/spidernet-io/cni-plugins/issues/177)等。

        我们通过推动社区修复此了问题，最终在 [only drop invalid cstate packets if non liberal](https://github.com/kubernetes/kubernetes/pull/120412) 得到解决，kubernetes 版本为 v1.29。我们需要确保设置每个节点的 sysctl 参数: `sysctl -w net.netfilter.nf_conntrack_tcp_be_liberal=1`，并且重启 Kube-proxy，这样 kube-proxy 就不会下发这条 drop 规则，也就不会影响到单 Macvlan pod 与单 Calico pod 之间的通信。

        执行完毕后，检查节点是否还存在这条 drop 规则，如果没有输出说明正常。否则请检查 sysctl 是否正确设置以及是否重启 kube-proxy。

        ~# iptables-save -t filter | grep '--ctstate INVALID -j DROP'

        注意： 必须确保 k8s 版本大于 v1.29。如果您的 k8s 版本小于 v1.29, 那么这条规则将会影响 Macvlan Pod 与 Calico Pod 之间的访问。

4. 当不存在这条 drop 规则，pod1 发出的回复数据包按照线路 `6` 能够直接将数据包转发到 pod3，整个访问结束。

### Macvlan Pod 访问 Calico Pod 的 Service

如图 1 中 序号 `3`, `4` 和 `5` 的线路所示:

1. Spiderpool 会在 Pod3 (单 macvlan 网卡 pod) 内注入一条 service 的路由, 期望 Pod3 按照线路 `3` 访问 Service 的时候，数据包通过 veth0 网卡转发到节点 node3。10.233.0.0/18 为 Service 的子网:

        ～# ip r
        10.233.0.0/18 via 10.7.168.71 dev eth0 src 10.233.100.2

2. 当数据包转发到节点 node3 之后，经过主机网络协议栈的 kube-proxy 将 clusterip 转换为 Pod1(单 calico 网卡的 pod) 的 IP, 随后通过 Calico 设置的隧道路由转发到节点 node1。注意当请求数据包从节点 node3 发出时，其源地址会被 SNAT 为的节点 node3 IP，这确保目标主机 node1 收到数据包时，能够将数据包原路返回，而不会出现上个场景的来回路径不一致的问题。这样请求数据包转发到主机 node1 后，通过 calixxx 虚拟网卡转发到了 pod1。

3. Pod1(单 calico 网卡 pod) 按照线路 `4` 通过 calixxx 虚拟网卡将回复报文转发到节点 node1。此时回复数据包的目标地址为节点 node3 的 IP，所以通过节点路由转发到 node3。随后通过 Kube-proxy 将源地址改回为 Pod3 (单 macvlan 网卡 pod) 的 IP，随后匹配 spiderpool 在主机上设置的 macvlan pod 直连路由，按照线路 `5` 通过 vethxxx 设备转发到目标 Pod3，整个访问完成。

## 结论

我们总结了这三种类型的 Pod 存在于一个集群时的一些通信场景，如下:

| 源\目标 | Calico Pod | Macvlan Pod | Calico + Macvlan 多网卡 Pod | Calico Pod 的 Service | Macvlan Pod 的 Service | Calico + Macvlan 多网卡 Pod 的 Service |
|-|-|-|-|-|-|-|
| Calico Pod | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Macvlan Pod | 要求 kube-proxy 的版本大于 v1.29 | ✅ | ✅ | ✅ | ✅ | ✅ |
| Calico + Macvlan 多网卡 Pod | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
