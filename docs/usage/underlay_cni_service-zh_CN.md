# Underlay CNI 访问 Service

**简体中文** | [**English**](./underlay_cni_service.md)

## 介绍

目前社区中大多数 Underlay 类型的 CNI(如 Macvlan、IPVlan、Sriov-CNI 等)一般对接底层网络，往往并不原生支持访问集群的 Service 。这大多是因为 underlay Pod 访问 Service 需要经过交换机的网关转发，
但网关上并没有去往 Service 的路由，造成无法正确路由访问 Service 的报文，从而丢包。Spiderpool 提供以下两种的方案解决 Underlay CNI 访问 Service 的问题:

- 通过 `kube-proxy` 访问 Service
- 通过 `cgroup eBPF 实现 service` 访问 Service

这两种方案都解决了 Underlay CNI 无法访问 Service 的问题，但实现原理有些不同。下面我们将介绍这两种方式:

## 基于 kube-proxy 实现 service 访问

Spiderpool 内置 `coordinator` 插件，它可以帮助我们无缝对接 `kube-proxy` 以实现 Underlay CNI 访问 Service。 根据不同的场景，`coordinator` 可以运行在 `underlay` 或 `overlay` 模式，虽然实现方式稍显不同，但
核心原理都是将 Pod 访问 Service 的流量劫持的主机网络协议栈上，再经过 Kube-proxy 创建的 IPtables 规则做转发。

下面是数据转发流程图介绍:

![service_kube_proxy](../images/spiderpool_service_kube_proxy.png)

- 对于 coordinator 运行在 `Underlay`

在此模式下，`coordinator` 插件将创建一对 Veth 设备，将一端放置于主机，另一端放置与 Pod 的 network namespace 中，然后在 Pod 里面设置一些路由规则， 使 Pod 访问 ClusterIP 时从 veth 设备转发。 `coordinator` 默认为 auto 模式，
它将自动判断应该运行 `underlay` 或 `overlay` 模式。您只需要在 Pod 注入注解: `v1.multus-cni.io/default-network: kube-system/<Multus_CR_NAME>` 即可。

当以 Underlay 模式创建 Pod 后，我们进入到 Pod 内部，看看路由等信息:

```shell
root@controller:~# kubectl exec -it macvlan-underlay-5496bb9c9b-c7rnp sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
#
# ip a show veth0
5: veth0@if428513: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 4a:fe:19:22:65:05 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet6 fe80::48fe:19ff:fe22:6505/64 scope link
       valid_lft forever preferred_lft forever
# ip r
default via 10.6.0.1 dev eth0
10.6.0.0/16 dev eth0 proto kernel scope link src 10.6.212.241
10.6.212.101 dev veth0 scope link
10.233.64.0/18 via 10.6.212.101 dev veth0
```

- **10.6.212.101 dev veth0 scope link**: 10.6.212.101 是节点的 IP,确保 Pod 访问本节点时从 `veth0` 转发。
- **10.233.64.0/18 via 10.6.212.101 dev veth0**: 10.233.64.0/18 是集群 Service 的 CIDR, 确保 Pod 访问 ClusterIP 时从 `veth0` 转发。

这个方案强烈依赖与 kube-proxy 的 MASQUERADE , 否则回复报文将直接转发给源 Pod, 如果经过一些安全设备，将会丢弃数据包。所以在一些特殊的场景下，我们需要设置 kube-proxy 的 `masqueradeAll` 为 true。

> 在默认情况下，Pod 的 underlay 子网与集群的 clusterCIDR 不同， 无需开启 `masqueradeAll`, 它们之间的访问将会被 SNAT。
>
> 如果 Pod 的 underlay 子网与集群的 clusterCIDR 相同，那我们必须要设置 `masqueradeAll` 为 true。

### 对于 `coordinator` 运行在 Overlay 模式

配置 `coordinator` 为 Overlay 模式同样也能解决 Underlay CNI 访问 Service 的问题。 传统的 Overlay 类型(如 [Calico](https://github.com/projectcalico/calico) 和 [Cilium](https://github.com/cilium/cilium) 等)的
CNI 已经完美解决了访问 Service 的问题。 我们可以借助它，帮助 Underlay Pod 访问 Service。 我们可以为 Pod 附加多张网卡，`eth0` 为 Overlay CNI 创建，用于转发集群东西向流量。`net1` 为 Underlay CNI 创建，用于转发 Pod 南北向流量。
通过 `coordinator` 设置的策略路由表项 确保 Pod 访问 Service 时从 eth0 转发, 回复报文也转发给 eth0。

> 在默认情况下 mode 的值为auto(spidercoordinator CR 中 spec.mode 为 auto), `coordinator` 将通过对比当前 CNI 调用网卡是否不是 `eth0`。如果不是，确认 Pod 中不存在 `veth0` 网卡，则自动判断为 overlay 模式。
>
> 在 overlay 模式下，Spiderpool 会自动同步集群缺省 CNI 的 Pod 子网，这些子网用于在多网卡 Pod 中设置路由，以实现它访问由缺省 CNI 创建的 Pod 之间的正常通信时，从 `eth0` 转发。这个配置对应 `spidercoordinator.spec.podCIDRType`，默认为 `auto`, 可选值: ["auto","calico","cilium","cluster","none"]
>
> 这些路由是在 Pod 启动时注入的，如果相关的 CIDR 发生了变动，无法自动生效到已经 Running 的 Pod 中，这需要重启 Pod 才能生效。
>
> 更多详情参考 [CRD-Spidercoordinator](../reference/crd-spidercoordinator.md)

当以 Overlay 模式创建 Pod 并进入 Pod 网络命令空间，查看路由信息:

```shell
root@controller:~# kubectl exec -it macvlan-overlay-97bf89fdd-kdgrb sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
#
# ip rule
0: from all lookup local
32759: from 10.233.105.154 lookup 100
32766: from all lookup main
32767: from all lookup default
# ip r
default via 169.254.1.1 dev eth0
10.6.212.102 dev eth0 scope link
10.233.0.0/18 via 10.6.212.102 dev eth0
10.233.64.0/18 via 10.6.212.102 dev eth0
169.254.1.1 dev eth0 scope link
# ip r show table 100
default via 10.6.0.1 dev net1
10.6.0.0/16 dev net1 proto kernel scope link src 10.6.212.227
```

- **32759: from 10.233.105.154 lookup 100**: 确保从 `eth0` (calico 网卡)发出的数据包走 table 100。
- 默认情况下: 除了默认路由，所有路由都保留在 Main 表，但会把 net1 的默认路由移动到 table 100。

这些策略路由确保多网卡场景下，Underlay Pod 也能够正常访问 Service。

## 基于 cgroup eBPF 实现 service 访问

上面我们介绍了在 Spiderpool 中, 我们通过 `coordinator` 将 Pod 访问 Service 的流量劫持到主机转发， 再经过主机上 Kube-proxy 设置的 iptables 规则 DNAT (将目标地址改为目标 Pod) 之后，再转发至目标 Pod。
这可以虽然解决问题，但可能延长了数据访问路径，造成一定的性能损失。

社区开源的 CNI 项目: Cilium 支持基于 eBPF 技术完全替代 kube-proxy 系统组件。可以帮助我们解析 Service。当访问 Service 时，Service
地址会被 Cilium 挂载的 eBPF 程序直接解析为目标 Pod 的 IP，这样源 Pod 直接对目标 Pod 发起访问，而不需要经过主机的网络协议栈，极大的缩短了访问路径，从而实现加速访问 Service。借助于强大的 Cilium，
我们也可以通过它实现加速 Underlay CNI的 Service 访问。

![cilium_kube_proxy](../images/withou_kube_proxy.png)

经过测试，相比 kube proxy 解析方式，cgroup eBPF 方式的[网络延时有最大 25% 的改善，网络吞吐有 50% 的提高](../concepts/io-performance-zh_CN.md) 。

以下步骤演示在具备 2 个节点的集群上，如何基于 Macvlan CNI + Cilium 加速访问 Service：

> 注意: 需要确保集群节点的内核版本至少大于 4.19

提前准备好一个未安装 kube-proxy 组件的集群，如果已经安装 kube-proxy 可参考一下命令删除 kube-proxy 组件

```shell
~# kubectl delete ds -n kube-system kube-proxy
~# # 在每个节点上运行
~# iptables-save | grep -v KUBE | iptables-restore
```

也可以使用 kubeadm 安装一个新集群, 注意不要安装 kube-proxy:

```shell
~# kubeadm init --skip-phases=addon/kube-proxy
```

安装 Cilium 组件，注意开启 kube-proxy replacement 功能

```shell
~# helm repo add cilium https://helm.cilium.io
~# helm repo update
~# API_SERVER_IP=<your_api_server_ip>
~# # Kubeadm default is 6443
~# API_SERVER_PORT=<your_api_server_port>
~# helm install cilium cilium/cilium --version 1.14.3 \
  --namespace kube-system \
  --set kubeProxyReplacement=true \
  --set k8sServiceHost=${API_SERVER_IP} \
  --set k8sServicePort=${API_SERVER_PORT}
```

安装完成，检查安装状态：

```shell
~# kubectl  get po -n kube-system | grep cilium
cilium-2r6s5                             1/1     Running     0              15m
cilium-lr9lx                             1/1     Running     0              15m
cilium-operator-5ff9f86dfd-lrk6r         1/1     Running     0              15m
cilium-operator-5ff9f86dfd-sb695         1/1     Running     0              15m
```

安装 Spiderpool, 可参考 [安装](./install/underlay/get-started-macvlan-zh_CN.md) 安装 Spiderpool:

```shell
~# helm install spiderpool spiderpool/spiderpool -n kube-system \
    --set multus.multusCNI.defaultCniCRName="macvlan-conf" \
    --set  coordinator.podCIDRType=none
```

> 设置 coordinator.podCIDRType=none, spiderpool 将不会获取集群的 ServiceCIDR。在创建 Pod 时也就不会注入 Service 相关路由
>
> 这样访问 Service 完全依赖 Cilium kube-proxy Replacement。
>
> 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。

完成后，安装的组件如下:

```shell
~# kubectl get pod -n kube-system
spiderpool-agent-9sllh                         1/1     Running     0          1m
spiderpool-agent-h92bv                         1/1     Running     0          1m
spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
spiderpool-init                                0/1     Completed   0          1m
```

创建 macvlan 相关的 multus 配置，并创建配套的 ippool 资源:

```shell
~# cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: v4-pool
spec:
  gateway: 172.81.0.1
  ips:
  - 172.81.0.100-172.81.0.120
  subnet: 172.81.0.0/16
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-ens192
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - "ens192"
    ippools:
      ipv4: ["v4-pool"]
EOF
```

> 需要确保 ens192 存在于集群节点
>
> 建议设置 enableCoordinator 为 true, 这可以解决 Pod 健康检测的问题

创建一组跨节点的 DaemonSet 应用用于测试：

```yaml
ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/macvlan-ens192"
NAME=ipvlan
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ${NAME}
  labels:
    app: $NAME
spec:
  selector:
    matchLabels:
      app: $NAME
  template:
    metadata:
      name: $NAME
      labels:
        app: $NAME
      annotations:
        ${ANNOTATION_MULTUS}
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: ${NAME}
spec:
  - ports:
    name: http
    port: 80
    protocol: TCP
    targetPort: 80
  selector:
    app: ${NAME}
  type: ClusterIP
EOF
```

验证访问 Service 的联通性，并查看性能是否提升

```shell
~# kubectl exec -it ipvlan-test-55c97ccfd8-kd4vj sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
/ # curl 10.233.42.25 -I
HTTP/1.1 200 OK
Server: nginx
Date: Fri, 20 Oct 2023 07:52:13 GMT
Content-Type: text/html
Content-Length: 4055
Last-Modified: Thu, 02 Mar 2023 10:57:12 GMT
Connection: keep-alive
ETag: "64008108-fd7"
Accept-Ranges: bytes
```

另开一个终端，进入到 Pod 的网络命名空间，通过 tcpdump 工具查看到访问 Service 的数据包从 Pod 网络命名空间发出时，目标地址已经被解析为目标 Pod 地址:

```shell
~# tcpdump -nnev -i eth0 tcp and port 80
tcpdump: listening on eth0, link-type EN10MB (Ethernet), capture size 262144 bytes
10.6.185.218.43550 > 10.6.185.210.80: Flags [S], cksum 0x87e7 (incorrect -> 0xe534), seq 1391704016, win 64240, options [mss 1460,sackOK,TS val 2667940841 ecr 0,nop,wscale 7], length 0
10.6.185.210.80 > 10.6.185.218.43550: Flags [S.], cksum 0x9d1a (correct), seq 2119742376, ack 1391704017, win 65160, options [mss 1460,sackOK,TS val 3827707465 ecr 2667940841,nop,wscale 7], length 0 
```

> `10.6.185.218` 是源 Pod 的 IP, `10.6.185.210` 是目标 Pod 的 IP，可以确认 Cilium 解析了 Service 的 IP。

使用 sockperf 工具测试使用 Cilium 加速前后， 测得 Pod 跨节点访问 ClusterIP 的数据对比:

|   | latency(usec) | RPS |
|---|---|---|
| with kube-proxy | 36.763 | 72254.34 |
| without kube-proxy | 27.743 | 107066.38 |

根据结果显示，经过 Cilium kube-proxy replacement 之后，访问 Service 大约加速 30%。更多测试数据参考[网络 IO 性能](../concepts/io-performance-zh_CN.md)

## 结论

Underlay CNI 访问 Service 有以上两种方案解决。kube-proxy 的方式更加常用稳定，大部分环境都可以稳定使用。 cgroup eBPF 为 Underlay CNI 访问 Service 提供了另一种可选方案，并且加速了 Service 访问，尽管这有一定使用限制及门槛，但在特定场景下能够满足用户的需求。
