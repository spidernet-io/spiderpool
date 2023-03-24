# 云原生网络新玩法：一种支持固定多网卡IP的 underlay 网络解决方案

随着数据中心的私有云的发展，应用希望能直接获取宿主机网络中的 IP 地址，直接使用该 IP 地址实现在 Underlay 网络下的东西向和南北向通信；同时有对 IPAM 的特殊需求，往往会产生如下的一些诉求，并迫切希望能得到解决：

* Pod 多网卡支持。
  1. 为使 Pod 能通达不同的 Underlay 子网，需给应用的不同网卡分配不同子网下的 IP 地址，社区现有的方案配合 Multus 能分配多网卡 IP 地址，但是不能实现多网卡都固定 IP 地址。
  2. 社区现有的多网卡分配 IP 地址方案，还存在多网卡路由协调问题；默认路由在网卡 1 上，外部访问网卡 2，通过默认路由从而走到网卡 1，这将会导致网络不通。

* 有状态和无状态应用 IP 固定。
  1. Pod 的 IP 地址常常受防火墙策略管控，防火墙只会允许特定的 IP 或者 IP 范围内的目标访问。
  2. 有状态应用的特殊性，固定 IP 地址可以保证应用的可用性、稳定性和可靠性。
  3. 传统微服务应用直接使用 Pod IP 进行微服务注册，对固定 IP 地址的需求。传统应用在云化改造前，是部署在裸金属环境上的，服务之间的网络未引入 NAT 地址转换，微服务架构中需要感知对方的源 IP 或目的 IP。

* 应用固定 IP 地址数量的弹性扩缩容。
  1. 应用固定 IP 场景下，IP 地址数量的扩缩容问题。随着应用扩缩容，如果能够实现固定 IP 地址一起扩缩，将避免每次都需要去人工修改 IP 固定池。
  2. 应用固定 IP 场景下，IP 地址数量的冗余问题。应用在滚动更新时，新 Pod 副本先启动，才会删除旧 Pod，在此过程中，如果没有冗余的固定 IP 地址，那么新 Pod 副本会因为缺少新的固定 IP 而启动失败。

以下列举了一些开源社区提供的对接 Underlay 网络的 CNI 方案：

* Kube-ovn：
  - 无法实现 CRD 化的固定 IP 管理。
  - 应用扩缩容时，需要人为添加或删除 IP 地址。
  - 应用滚动发布时，新建的 Pod 可能会面临没有临时 IP 可用，从而导致发布失败。

* Antrea：
  - 不支持 Deployment/Statefulset 类型的 IP 地址固定。可参考 [Antrea 文档](https://github.com/antrea-io/antrea/blob/main/docs/antrea-ipam.md#ippool-annotations-on-pod-available-since-antrea-15)。 

* Calico BGP:
  - 固定 IP 只在 Pod 级别生效，不支持 Deployment/Statefulset 类型的 IP 地址固定。


随着对开源社区的探索，了解到一款全新的开源项目 Spiderpool，而什么是 Spiderpool？SpiderPool 是一个 Kubernetes 的 IPAM 插件项目, 其主要针对于 Underlay 网络的 IP 地址管理需求而设计, 能够为任何兼容第三方 IPAM 插件的 CNI 项目所使用，它克服了 Underlay 网络分配 IP 地址的复杂性，使得 IP 分配的运维工作像一些 overlay 网络模型一样简单，同时它包括应用 IP 地址固定、IP 地址自动弹性扩缩容、 多网卡、双栈支持等特点。更多说明参考 [SpiderPool](https://github.com/spidernet-io/spiderpool) 介绍。


通过 Spiderpool 搭配 Multus、Macvlan、Veth 探索出一种 "云原生网络新玩法：一种支持固定多网卡IP的 Underlay 网络解决方案"，该方案也可以搭配 [Calico](https://github.com/projectcalico/calico)、[SRI-OV](https://github.com/k8snetworkplumbingwg/sriov-cni)、[ipvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan)、[vlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan)、[ovs](https://github.com/k8snetworkplumbingwg/ovs-cni) 等 CNI，其中：

[`Multus`](https://github.com/k8snetworkplumbingwg/multus-cni) 是一个 CNI 插件项目，它通过调度第三方 CNI 项目，能够实现为 Pod 接入多张网卡。并且，Multus 提供了 CRD 方式管理 Macvlan 的 CNI 配置，避免在每个主机上手动编辑 CNI 配置文件。

[`Macvlan`](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) : 能够为 Pod 分配 Macvlan 虚拟网卡，可用于对接 Underlay 网络。

[`Veth`](https://github.com/spidernet-io/plugins) : 是一个 CNI 插件，它能够帮助一些 CNI （例如 Macvlan、SR-IOV 等）解决如下问题：

* 在 Macvlan CNI 场景下，帮助 Pod 实现 clusterIP 通信

* 若 Pod 的 Macvlan IP 不能与本地宿主机通信，会影响 Pod 的健康检测。Veth 插件能够帮助 Pod 与宿主机通信，解决健康检测场景下的联通性问题

* 在 Pod 多网卡场景下，Veth 能自动够协调多网卡间的策略路由，解决多网卡通信问题


该解决方案的网络拓扑图如下:

![multus_macvlan_veth_spiderpool](../images/multus_macvlan_veth_spiderpool.svg)


## 安装 

以上提及的 Multus 、Macvlan、Veth 的安装方法，请参考链接 [安装](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/get-started-macvlan-zh_CN.md)：

### 安装 Spiderpool CRD

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

helm repo update spiderpool

helm install spiderpool spiderpool/spiderpool --namespace kube-system \
    --set feature.enableIPv4=true --set feature.enableIPv6=false
```

### 创建 SpiderSubnet 实例

本次示例中为使用 Spiderpool 多网卡固定 IP 的功能，需要创建 ens192、ens224 网卡的底层 Underlay 子网，供 Pod 使用。以下是创建相关 SpiderSubnet 实例的示例：

```shell
ENS192_EXPECT_IP_RANGE="10.6.168.171-10.6.168.180"
ENS192_EXPECT_SUBNET="10.6.0.1/16"
ENS192_EXPECT_GATEWAY="10.6.0.1"
ENS224_EXPECT_IP_RANGE="10.7.168.171-10.7.168.180"
ENS224_EXPECT_SUBNET="10.7.0.1/16"
ENS224_EXPECT_GATEWAY="10.7.0.1"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: subnet-test-ens192
spec:
  ipVersion: 4
  ips:
  - "${ENS192_EXPECT_IP_RANGE}"
  subnet: "${ENS192_EXPECT_SUBNET}"
  gateway: "${ENS192_EXPECT_GATEWAY}"
  vlan: 0
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: subnet-test-ens224
spec:
  ipVersion: 4
  ips:
  - "${ENS224_EXPECT_IP_RANGE}"
  subnet: "${ENS224_EXPECT_SUBNET}"
  gateway: "${ENS224_EXPECT_GATEWAY}"
  vlan: 0
EOF
```

查看 SpiderSubnet 实例的创建情况

```bash
~# kubectl get spidersubnet
NAME                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-test-ens192   4         10.6.0.1/16   3                    10
subnet-test-ens224   4         10.7.0.1/16   3                    10
```

### 创建 Macvlan 的 Multus NetworkAttachmentDefinition 配置

为使用 Spiderpool 多网卡固定 IP 的功能，需使用下列命令为 Macvlan 创建两个 Multus 的 NetworkAttachmentDefinition 配置

   需要确认如下参数：

   * 确认 Macvlan 所需的宿主机父接口，本次示例以宿主机的 ens192、ens224 网卡为例，创建 Macvlan 子接口给 Pod 使用

   * 为使用 Veth 插件来实现 clusterIP 通信，需确认集群 service 的 serviceIP CIDR，例如可基于命令 `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service` 查询
      
```shell
MACVLAN_MASTER_INTERFACE_ENS192="ens192"
MACVLAN_MASTER_INTERFACE_ENS224="ens224"
SERVICE_CIDR="10.96.0.0/12"

cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf-ens192
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-conf-ens192",
        "plugins": [
            {
                "type": "macvlan",
                "master": "${MACVLAN_MASTER_INTERFACE_ENS192}",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                  "type": "veth",
                  "service_cidr": ["${SERVICE_CIDR}"]
              }
        ]
    }
---
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf-ens224
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-conf-ens224",
        "plugins": [
            {
                "type": "macvlan",
                "master": "${MACVLAN_MASTER_INTERFACE_ENS224}",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                  "type": "veth",
                  "service_cidr": ["${SERVICE_CIDR}"]
              }
        ]
    }
EOF
```

## 固定多网卡 IP

创建 Deployment，并通过 Spiderpool 为其自动创建多网卡固定 IP 池，其中：

  * `ipam.spidernet.io/subnets`： 通过指定子网，在子网中随机选择一些 IP 为应用 `test-app` 自动创建固定 IP 池，并且与应用绑定，实现 IP 固定。
  * `ipam.spidernet.io/ippool-ip-number`： 使应用创建的 IP 池的 IP 数量可以是固定，也可以是弹性扩缩的（`+1`：表示自动分配的固定 IP 数量比应用副本数多 1 个，`+0`：表示自动分配的固定 IP 数量与应用副本数相同。）
  * `v1.multus-cni.io/default-network`：为应用 `test-app` 创建一张默认网卡。
  * `k8s.v1.cni.cncf.io/networks`: 为应用 `test-app` 创建另一张网卡。

以下的示例 Yaml 中， 会创建 2 个副本的 Deployment，2 个属于不同 Underlay 子网的固定 IP 池，其池中 IP 数量是：`3`。

```shell
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnets: |-
         [
            {      
              "interface": "eth0",      
              "ipv4": [       
                "subnet-test-ens192"      
              ]
            },{      
              "interface": "net1",      
              "ipv4": [       
                "subnet-test-ens224"      
              ]
            }
         ]
        ipam.spidernet.io/ippool-ip-number: '+1'
        v1.multus-cni.io/default-network: kube-system/macvlan-conf-ens192
        k8s.v1.cni.cncf.io/networks: kube-system/macvlan-conf-ens224
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

最终, 在 Deployment 创建时，Spiderpool 会随机从指定子网中选择一些 IP 来创建固定 IP 池，Deployment Pod 的 IP 会从固定 IP 池中分配，SpiderPool 从 annnotation 指定的 `子网: subnet-test-ens192` 中自动创建了一个名为 `auto-test-app-v4-eth0-b1a361c7e9df` 的 IP 池，从`子网: subnet-test-ens224` 中自动创建了一个名为 `auto-test-app-v4-net1-b1a361c7e9df` 的 IP 池，并与应用绑定，固定池 IP 数量为 `3`。

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-6f4594ff67-fkqbw   1/1     Running   0          40s   10.6.168.172   node2   <none>           <none>
test-app-6f4594ff67-gwlx8   1/1     Running   0          40s   10.6.168.173   node1   <none>           <none>

~# kubectl get spiderippool

NAME                                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-test-app-v4-eth0-b1a361c7e9df   4         10.6.0.1/16   2                    3                false     false
auto-test-app-v4-net1-b1a361c7e9df   4         10.7.0.1/16   2                    3                false     false

~# kubectl get spiderippool auto-test-app-v4-eth0-b1a361c7e9df -o jsonpath='{.spec.ips}'

["10.6.168.171-10.6.168.173"]

~# kubectl get spiderippool auto-test-app-v4-net1-b1a361c7e9df -o jsonpath='{.spec.ips}'

["10.7.168.171-10.7.168.173"]
```

如下命令展示了 Pod 的网卡信息，eth0 与 net1 网卡分别分配了 IP 池 `auto-test-app-v4-eth0-b1a361c7e9df` 和 `auto-test-app-v4-net1-b1a361c7e9df` 中的固定 IP，符合预期。

```bash
~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip a

1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN group default qlen 1000
    link/ipip 0.0.0.0 brd 0.0.0.0
3: eth0@if2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether ae:fa:5e:d9:79:11 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.6.168.172/16 brd 10.6.255.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::acfa:5eff:fed9:7911/64 scope link
       valid_lft forever preferred_lft forever
4: veth0@if13: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 26:6f:22:91:22:f9 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet6 fe80::246f:22ff:fe91:22f9/64 scope link
       valid_lft forever preferred_lft forever
5: net1@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether d6:4b:c2:6a:62:0f brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.7.168.173/16 brd 10.7.255.255 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fe80::d44b:c2ff:fe6a:620f/64 scope link
       valid_lft forever preferred_lft forever
```

如下命令展示了 Pod 中的多网卡路由信息，Veth 插件能自动够协调多网卡间的策略路由，解决多网卡间的通信问题。

```bash
~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip r show

default via 10.6.0.1 dev eth0
10.6.0.0/16 dev eth0 proto kernel scope link src 10.6.168.172
10.6.168.123 dev veth0 scope link
10.96.0.0/12 via 10.6.168.123 dev veth0

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip rule show

0:	from all lookup local
32764:	from 10.7.168.173 lookup 100
32765:	from all to 10.7.168.173/16 lookup 100
32766:	from all lookup main
32767:	from all lookup default

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip route show table 100

default via 10.7.0.1 dev net1
10.6.168.123 dev veth0 scope link
10.7.0.0/16 dev net1 proto kernel scope link src 10.7.168.173
10.96.0.0/12 via 10.6.168.123 dev veth0
```

当重启 Pod 后，其 IP 仍都被固定在 `auto-test-app-v4-eth0-b1a361c7e9df` 与 `auto-test-app-v4-eth0-b1a361c7e9df` 的 IP 池范围内:

```bash
~# kubectl delete po -l app=test-app

~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-6f4594ff67-5lhgz   1/1     Running   0          8s    10.6.168.171   node1   <none>           <none>
test-app-6f4594ff67-kd99s   1/1     Running   0          8s    10.6.168.173   node2   <none>           <none>

```

## 固定 IP 池弹性扩缩容

创建 Deployment 时指定了注解 `ipam.spidernet.io/ippool-ip-number`: '+1'，其表示应用分配到的固定 IP 数量比应用的副本数多 1 个，在应用滚动更新时，能够避免旧 Pod 未删除，新 Pod 没有可用 IP 的问题。以下演示了扩容场景，将应用的副本数从 2 扩容到 3，应用对应的两个固定 IP 池会自动从 3 个 IP 扩容到 4 个 IP:

```bash
~# kubectl scale deploy test-app --replicas 3

~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-6f4594ff67-5lhgz   1/1     Running   0          35s   10.6.168.171   node1   <none>           <none>
test-app-6f4594ff67-kd99s   1/1     Running   0          35s   10.6.168.173   node2   <none>           <none>
test-app-6f4594ff67-kxmjd   1/1     Running   0          8s    10.6.168.172   node1   <none>           <none>

~# kubectl get spiderippool
NAME                                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-test-app-v4-eth0-b1a361c7e9df   4         10.6.0.1/16   3                    4                false     false
auto-test-app-v4-net1-b1a361c7e9df   4         10.7.0.1/16   3                    4                false     false
```

通过上述，Spiderpool 对于应用扩缩容的场景，只需要修改应用的副本数即可。

## Spiderpool 手动建池实现多网卡固定 IP

在需要防火墙等手段来精细管控网络安全场景下，网络管理员希望自己来直接指定应用的固定 IP 地址，而不是由 SpiderPool 自动从子网中随机选择 IP 。对此 SpiderPool 提供注解 `ipam.spidernet.io/ippool` 与 `ipam.spidernet.io/ippools` 能手动为应用绑定指定的 IP 池，但手动指定池将不支持自动扩缩容。对于 Spiderpool 的更多功能，请参考 [SpiderPool](https://github.com/spidernet-io/spiderpool) 。

## 结论

经过测试：Pod 能够通过 Pod IP、clusterIP、nodePort 等方式通信，在 Underlay 网络下，Spiderpool 搭配 Multus 、Macvlan、Veth 能支持多网卡固定 IP 地址的需求，这为在 Underlay 网络下解决多网卡固定 IP 地址提供了一种全新的方案。


参考链接:

* `https://spidernet-io.github.io/spiderpool`
* `https://github.com/k8snetworkplumbingwg/multus-cni`
* `https://github.com/spidernet-io/plugins`
* `https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan`
