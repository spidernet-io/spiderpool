# Ovs-cni Quick Start

[**English**](./get-started-ovs.md) | **简体中文**

Spiderpool 可用作 Underlay 网络场景下提供固定 IP 的一种解决方案，本文将以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Ovs-cni](https://github.com/k8snetworkplumbingwg/ovs-cni) 、[Spiderpool](https://github.com/spidernet-io/spiderpool) 为例，搭建一套完整的 Underlay 网络解决方案，该方案能将可用的网桥公开为节点资源，供集群使用。

## 先决条件

1. 一个多节点的 Kubernetes 集群

2. [Helm 工具](https://helm.sh/docs/intro/install/)

3. 必须在主机上安装并运行 [Open vSwitch](https://docs.openvswitch.org/en/latest/intro/install/#installation-from-packages)
    
    以下示例是基于 Ubuntu 22.04.1。主机系统不同，安装方式可能不同。

    ```bash
    ~# sudo apt-get install -y openvswitch-switch
    ~# sudo systemctl start openvswitch-switch
    ```

## 安装 Ovs-cni 

[`ovs-cni`](https://github.com/k8snetworkplumbingwg/ovs-cni) 是一个基于 Open vSwitch（OVS）的 Kubernetes CNI 插件，它提供了一种在 Kubernetes 集群中使用 OVS 进行网络虚拟化的方式。

确认节点上是否存在二进制文件 /opt/cni/bin/ovs 。如果节点上不存在该二进制文件，可参考如下命令，在所有节点上下载安装：

```bash
~# wget https://github.com/k8snetworkplumbingwg/ovs-cni/releases/download/v0.31.1/plugin

~# mv ./plugin /opt/cni/bin/ovs

~# chmod +x /opt/cni/bin/ovs
```

Ovs-cni 不会配置网桥，由用户创建它们，并将它们连接到 L2、L3 网络。以下是创建网桥的示例，请在每个节点上执行：

1. 创建 Open vSwitch 网桥。

    ```bash
    ~# ovs-vsctl add-br br1
    ```

2. 网络接口连接到网桥

    此过程取决于您的平台，以下命令只是示例说明，它可能会破坏您的系统。首先使用 `ip link show` 查询主机的可用接口，示例中使用主机上的接口：`eth0` 为例。 

    ```bash
    ~# ovs-vsctl add-port br1 eth0
    ~# ip addr add <IP地址>/<子网掩码> dev br1
    ~# ip link set br1 up
    ~# ip route add default via <默认网关IP> dev br1
    ```

创建后，可以在每个节点上查看到如下的网桥信息：

```bash
~# ovs-vsctl show
ec16d9e1-6187-4b21-9c2f-8b6cb75434b9
    Bridge br1
        Port eth0
            Interface eth0
        Port br1
            Interface br1
                type: internal
        Port veth97fb4795
            Interface veth97fb4795
    ovs_version: "2.17.3"
```

## 安装 Multus

[`Multus`](https://github.com/k8snetworkplumbingwg/multus-cni) 是一个 CNI 插件项目，它通过调度第三方 CNI 项目，能够实现为 Pod 接入多张网卡。并且 Multus 提供了 CRD 方式管理 Ovs-cni 的 CNI 配置，避免在每个主机上手动编辑 CNI 配置文件，能够降低运维工作量。

1. 通过 manifest 安装 Multus

    ```shell
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. 为 Ovs-cni 创建 Multus 的 NetworkAttachmentDefinition 配置

     需要确认如下参数：

    * 确认 ovs-cni 所需的宿主机网桥，例如可基于命令 `ovs-vsctl show` 查询，本例子以宿主机的网桥：`br1` 为例

```bash
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ovs-conf
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "ovs-conf",
        "plugins": [
            {
                "type": "ovs",
                "bridge": "br1",
                "ipam": {
                    "type": "spiderpool"
                }
            }
        ]
    }
EOF
```

## 安装 Spiderpool

1. 安装 Spiderpool。

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system
    ```
    
    > 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。


2. 创建 SpiderSubnet 实例。

    Pod 会从该子网中获取 IP，进行 Underlay 的网络通讯，所以该子网需要与接入的 Underlay 子网对应。
    
    以下是创建相关的 SpiderSubnet 示例：

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderSubnet
    metadata:
      name: subnet-test
    spec:
      ipVersion: 4
      ips:
        - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
    EOF
    ```

验证安装：

```bash
~# kubectl get po -n kube-system | grep spiderpool
spiderpool-agent-f899f                       1/1     Running   0             2m
spiderpool-agent-w69z6                       1/1     Running   0             2m
spiderpool-controller-5bf7b5ddd9-6vd2w       1/1     Running   0             2m
~# kubectl get spidersubnet
NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-test   4         172.18.0.0/16   0                    10
```

## 创建应用

以下的示例 Yaml 中， 会创建 2 个副本的 Deployment，其中：

* `ipam.spidernet.io/subnet`：用于指定 Spiderpool 的子网，Spiderpool 会自动在该子网中随机选择一些 IP 来创建固定 IP 池，与本应用绑定，能实现 IP 固定的效果。

* `v1.multus-cni.io/default-network`：用于指定 Multus 的 NetworkAttachmentDefinition 配置，会基于它为应用创建一张默认网卡。

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
        ipam.spidernet.io/subnet: |-
          {
            "ipv4": ["subnet-test"]
          }
        v1.multus-cni.io/default-network: kube-system/ovs-conf
      labels:
        app: test-app
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: test-app
              topologyKey: kubernetes.io/hostname
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

Spiderpool 自动为应用创建了 IP 固定池，应用的 IP 将会自动固定在该 IP 范围内：

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
test-app-6f8dddd88d-hstg7   1/1     Running   0          3m37s   172.18.30.131   ipv4-worker          <none>           <none>
test-app-6f8dddd88d-rj7sm   1/1     Running   0          3m37s   172.18.30.132   ipv4-control-plane   <none>           <none>

~# kubectl get spiderippool
NAME                                 VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-test-app-v4-eth0-9b208a961acd   4         172.18.0.0/16   2                    2                false     false

~#  kubectl get spiderendpoints
NAME                        INTERFACE   IPV4POOL                             IPV4               IPV6POOL   IPV6   NODE
test-app-6f8dddd88d-hstg7   eth0        auto-test-app-v4-eth0-9b208a961acd   172.18.30.131/16                     ipv4-worker
test-app-6f8dddd88d-rj7sm   eth0        auto-test-app-v4-eth0-9b208a961acd   172.18.30.132/16                     ipv4-control-plane
```

测试 Pod 与 Pod 的通讯情况，以跨节点 Pod 为例：

```shell
~#kubectl exec -ti test-app-6f8dddd88d-hstg7 -- ping 172.18.30.132 -c 2

PING 172.18.30.132 (172.18.30.132): 56 data bytes
64 bytes from 172.18.30.132: seq=0 ttl=64 time=1.882 ms
64 bytes from 172.18.30.132: seq=1 ttl=64 time=0.195 ms

--- 172.18.30.132 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.195/1.038/1.882 ms
```
