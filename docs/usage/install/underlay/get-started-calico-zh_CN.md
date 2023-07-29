# Calico Quick Start

[**English**](./get-started-calico.md) | **简体中文**

Spiderpool 可用作 Underlay 网络场景下，为 Deployment、StatefulSet 等类型应用提供固定 IP 功能的一种解决方案。 本文将介绍在 Calico + BGP 模式下: 搭建一套完整的 Underlay 网络环境，搭配 Spiderpool 实现应用的固定 IP 功能，该方案可满足:

* 应用分配到固定的 IP 地址

* IP 池能随着应用副本自动扩缩容

* 集群外客户端可直接跳过应用 IP 访问应用

## 先决条件

* 一个 **_Kubernetes_** 集群(推荐 k8s version > 1.22), 并安装 **_Calico_** 作为集群的默认 CNI。

    确认 calico 不配置使用 IPIP 或者 vxlan 隧道，因为本例将演示如何使用 calico 对接 underlay 网络。

    确认 calico 开启了 fullmesh 方式的 BGP 配置。

* Helm、Calicoctl 二进制工具

## 安装 Spiderpool

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm install spiderpool spiderpool/spiderpool --namespace kube-system 
```

> Spiderpool 默认 IPv4-Only, 如需启用 IPv6 请参考 [Spiderpool IPv6](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/ipv6.md)
> 
> 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。

创建 Pod 的子网(SpiderSubnet):

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: nginx-subnet-v4
  labels:  
    ipam.spidernet.io/subnet-cidr: 10-244-0-0-16
spec:
  ips:
  - 10.244.100.0-10.244.200.1
  subnet: 10.244.0.0/16
EOF
```

验证安装：

```shell
[root@master ~]# kubectl get po -n kube-system  | grep spiderpool
  spiderpool-agent-27fr2                     1/1     Running     0          2m
  spiderpool-agent-8vwxj                     1/1     Running     0          2m
  spiderpool-controller-bc8d67b5f-xwsql      1/1     Running     0          2m
  [root@master ~]# kubectl get ss
  NAME              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
  nginx-subnet-v4   4         10.244.0.0/16   0                    25602
```

## 配置 Calico BGP [可选]

本例希望 calico 以 underlay 方式工作，将 Spiderpool 子网(`10.244.0.0/16`)通过 BGP 协议宣告至 BGP Router，确保集群外的客户端可以通过 BGP Router 直接访问 Pod 真实的 IP 地址。
如果您并不需要集群外部可以直接访问到 pod ip，可忽略本步骤。

网络拓扑如下:

![calico-bgp](../../../images/calico-bgp.svg)

1. 配置机器外的一台主机作为 BGP Router

    本次示例将一台 _Ubuntu_ 服务器作为 BGP Router。需要前置安装 FRR:

    ```shell
    root@router:~# apt install -y frr
    ```

    FRR 开启 BGP 功能:

    ```shell
    root@router:~# sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons
    root@router:~# systemctl restart frr
    ```

    配置 FRR:

    ```shell
    root@router:~# vtysh
    router# config
    router(config)# router bgp 23000 
    router(config)# bgp router-id 172.16.13.1 
    router(config)# neighbor 172.16.13.11 remote-as 64512 
    router(config)# neighbor 172.16.13.21 remote-as 64512  
    router(config)# no bgp ebgp-requires-policy 
    ```

    > 配置解释:
    >
    > * Router 侧的 AS 为 `23000`, 集群节点侧 AS 为 `64512`。Router 与节点之间为 `ebgp`, 节点之间为 `ibgp`
    > * 需要关闭 `ebgp-requires-policy`, 否则 BGP 会话无法建立
    > * 172.16.13.11/21 为集群节点 IP
    >     
    > 更多配置参考 [frrouting](https://docs.frrouting.org/en/latest/bgp.html)。

2. 配置 Calico 的 BGP 邻居

    Calico 需要配置 `calico_backend: bird`, 否则无法建立 BGP 会话:

    ```shell
    [root@master1 ~]# kubectl get cm -n kube-system calico-config -o yaml
    apiVersion: v1
    data:
      calico_backend: bird
      cluster_type: kubespray,bgp
    kind: ConfigMap
    metadata:
      annotations:
        kubectl.kubernetes.io/last-applied-configuration: |
          {"apiVersion":"v1","data":{"calico_backend":"bird","cluster_type":"kubespray,bgp"},"kind":"ConfigMap","metadata":{"annotations":{},"name":"calico-config","namespace":"kube-system"}}
    creationTimestamp: "2023-02-26T15:16:35Z"
    name: calico-config
    namespace: kube-system
    resourceVersion: "2056"
    uid: 001bbd09-9e6f-42c6-9339-39f71f81d363
    ```

    本例节点的默认路由在 BGP Router, 节点之间不需要相互同步路由，只需要将其自身路由同步给 BGP Router，所以关闭 _Calico BGP Full-Mesh_:

    ```shell
    [root@master1 ~]# calicoctl patch bgpconfiguration default -p '{"spec": {"nodeToNodeMeshEnabled": false}}'
    ```

    创建 BGPPeer:

    ```shell
    [root@master1 ~]# cat << EOF | calicoctl apply -f -
    apiVersion: projectcalico.org/v3
    kind: BGPPeer
    metadata:
      name: my-global-peer
    spec:
      peerIP: 172.16.13.1
      asNumber: 23000
    EOF
    ```

    > peerIP 为 BGP Router 的 IP 地址
    > 
    > asNumber 为 BGP Router 的 AS 号

    查看 BGP 会话是否成功建立:

    ```shell
    [root@master1 ~]# calicoctl node status
    Calico process is running.
     
    IPv4 BGP status
    +--------------+-----------+-------+------------+-------------+
    | PEER ADDRESS | PEER TYPE | STATE |   SINCE    |    INFO     |
    +--------------+-----------+-------+------------+-------------+
    | 172.16.13.1  | global    | up    | 2023-03-15 | Established |
    +--------------+-----------+-------+------------+-------------+
     
    IPv6 BGP status
    No IPv6 peers found.
    ```

    更多 Calico BGP 配置, 请参考 [Calico BGP](https://docs.tigera.io/calico/3.25/networking/configuring/bgp)

## 创建同子网的 Calico IP 池

我们需要创建一个与 Spiderpool 子网 CIDR 相同的 Calico IP 池, 否则 Calico 不会宣告 Spiderpool 子网的路由:

```shell
cat << EOF | calicoctl apply -f -
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: spiderpool-subnet
spec:
  blockSize: 26
  cidr: 10.244.0.0/16
  ipipMode: Never
  natOutgoing: false
  nodeSelector: all()
  vxlanMode: Never
EOF
```

> cidr 需要对应 Spiderpool 的子网: `10.244.0.0/16`
> 
> 设置 ipipMode 和 vxlanMode 为: Never

## 切换 Calico 的 `IPAM` 为 Spiderpool

修改每个节点上 Calico 的 CNI 配置文件: `/etc/cni/net.d/10-calico.conflist`, 将 ipam 字段切换为 Spiderpool:

```json
"ipam": {
    "type": "spiderpool"
},
```

## 创建应用

以 Nginx 应用为例:

```shell
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnet: '{"ipv4":["nginx-subnet-v4"]}'
        ipam.spidernet.io/ippool-ip-number: '+3'
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

> `ipam.spidernet.io/subnet`: 从 "nginx-subnet-v4" SpiderSubnet 中分配固定 IP
> 
> `ipam.spidernet.io/ippool-ip-number`: '+3' 表示应用分配的固定 IP 数量比应用副本数多 3 个，用于应用滚动发布时有临时 IP 可用

当应用 Pod 被创建，Spiderpool 从 annnotations 指定的 `subnet: nginx-subnet-v4` 中自动创建了一个名为 `auto-nginx-v4-eth0-452e737e5e12` 的 IP 池，并与应用绑定。IP 范围为: `10.244.100.90-10.244.100.95`, 池 IP 数量为 `5`:

```shell
[root@master1 ~]# kubectl get sp
NAME                              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-nginx-v4-eth0-452e737e5e12   4         10.244.0.0/16   2                    5                false     false
[root@master ~]# kubectl get sp auto-nginx-v4-eth0-452e737e5e12 -o jsonpath='{.spec.ips}' 
["10.244.100.90-10.244.100.95"]
```

当副本重启，其 IP 都被固定在 `auto-nginx-v4-eth0-452e737e5e12` 的 IP 池范围内:

```shell
[root@master1 ~]# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE     IP              NODE      NOMINATED NODE   READINESS GATES
nginx-644659db67-szgcg   1/1     Running       0          23s     10.244.100.90    worker5   <none>           <none>
nginx-644659db67-98rcg   1/1     Running       0          23s     10.244.100.92    master1   <none>           <none>
```

扩容副本数到 `3`, 新副本的 IP 地址仍然从自动池: `auto-nginx-v4-eth0-452e737e5e12(10.244.100.90-10.244.100.95)` 中分配:

```shell
[root@master1 ~]# kubectl scale deploy nginx --replicas 3  # scale pods
deployment.apps/nginx scaled
[root@master1 ~]# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE     IP              NODE      NOMINATED NODE   READINESS GATES
nginx-644659db67-szgcg   1/1     Running       0          1m     10.244.100.90    worker5   <none>           <none>
nginx-644659db67-98rcg   1/1     Running       0          1m     10.244.100.92    master1   <none>           <none>
nginx-644659db67-brqdg   1/1     Running       0          10s    10.244.100.94    master1   <none>           <none>
```

查看自动池: `auto-nginx-v4-eth0-452e737e5e12` 的 `ALLOCATED-IP-COUNT` 和 `TOTAL-IP-COUNT` 都新增 1 :

```shell
[root@master1 ~]# kubectl get sp
NAME                              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-nginx-v4-eth0-452e737e5e12   4         10.244.0.0/16   3                    6                false     false
```

## 结论

经过测试: 集群外客户端可直接通过 Nginx Pod 的 IP 正常访问，集群内部通讯 Nginx Pod 跨节点也都通信正常(包括跨 Calico 子网)。在 Calico BGP 模式下，Spiderpool 可搭配 Calico 实现 Deployment 等类型应用固定 IP 的需求。
