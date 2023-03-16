# Quick Start

默认情况下, Calico 使用自身支持的 IPAM : `calico-ipam` 为容器提供 IP 地址的管理分配能力。本文将介绍在 Calico 使用 BGP 协议宣告 Pod 子网路由的情况下, 通过 SpiderPool 为 Calico 提供 IP 地址细腻度的分配管理能力。

## 预置条件

- 一个 Kubernetes 集群, 并使用 Calico 作为集群的 CNI
- 一个支持 BGP 协议的路由器 或 安装支持 BGP 协议软件(如 FRR、Bird等)的机器
- [SpiderPool](): 一款开源的 Kubernetes IPAM 项目
- Helm 二进制工具
- Calicoctl 二进制工具

## 安装 SpiderPool

1. 设置 Helm 仓库

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    ```

2. 配置 IP 池并安装 Spiderpool

   - 配置子网信息
   
   ```shell
   export IPV4_SUBNET_YOUR_EXPECT="10.244.0.0/16"
   export IPV4_IPRANGE_YOUR_EXPECT="10.244.100.0-10.244.105.200"
   ```
   
   > _IPV4_SUBNET_YOUR_EXPECT_: 配置 Pod 子网, 该子网用于给 Pod 分配 IP 地址
   > 
   > _IPV4_IPRANGE_YOUR_EXPECT_: 配置 Pod 子网范围, Pod 将会从此范围中分配 IP 地址
     
   - 使用以下命令安装 SpiderPool

   ```shell
   helm install spiderpool spiderpool/spiderpool --namespace kube-system \
   --set feature.enableIPv4=true \
   --set clusterDefaultPool.installIPv4IPPool=true  \
   --set clusterDefaultPool.ipv4Subnet=${IPV4_SUBNET_YOUR_EXPECT} \
   --set clusterDefaultPool.ipv4IPRanges={${IPV4_IPRANGE_YOUR_EXPECT}} 
   ```
   
   > 注意: 本示例不涉及 IPv6, 如需开启 IPv6 或更多配置请参考 [IPv6]()
    
3. 验证安装

   ```shell
   [root@node1 ~]# kubectl get po -n kube-system  | grep spiderpool
   spiderpool-agent-27fr2                     1/1     Running     0          2m
   spiderpool-agent-8vwxj                     1/1     Running     0          2m
   spiderpool-controller-bc8d67b5f-xwsql      1/1     Running     0          2m
   spiderpool-init                            0/1     Completed   1          2m
   [root@node1 ~]# kubectl get sp
   NAME               VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
   default-v4-ippool   4         10.244.0.0/16             191                  1481             false
   ```

## 基础网络配置

Calico 默认使用 Overlay 模式, 即采用 `Vxlan/IPIP` 协议封装 Pod 跨节点的流量。在本示例中, 
集群默认子网(`172.25.0.0/16`)采用 `Vxlan` 协议封装跨节点的流量, 并将 SpiderPool 子网(`10.244.0.0/16`)通过 BGP 协议宣告至 BGP Router。
这样集群外的客户端就可以直接访问 Pod 真实的 IP 地址。

本次示例网络拓扑图如下:

![](../images/calico-spiderpool.svg)
   
> 集群节点的网关为 BGP Router

注: 因无交换机和路由器作为 BGP Router, 本次示例将一台 Ubuntu 服务器作为 BGP Router。需要前置安装 FRR:

   ```shell
   root@router:~# apt install -y frr
   ```
FRR 开启 BGP 功能:

   ```shell
   root@router:~# sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons
   root@router:~# systemctl restart frr
   ```


1. BGP Router 配置 BGP  

   ```shell
   root@router:~# vtysh

   Hello, this is FRRouting (version 8.1).
   Copyright 1996-2005 Kunihiro Ishiguro, et al.

   router# config
   router(config)# router bgp 23000 
   router(config)# bgp router-id 172.16.13.1 
   router(config)# neighbor 172.16.13.11 remote-as 64512 
   router(config)# neighbor 172.16.13.21 remote-as 64512  
   router(config)# no bgp ebgp-requires-policy 
   ```
   
配置解释:

- Router 侧的 AS 为 `23000`, 集群节点侧 AS 为 `64512`。Router 与节点之间为 `ebgp`, 节点之间为 `ibgp`。
- 需要关闭 `ebgp-requires-policy`, 否则 BGP 会话无法建立。
- 更多配置参考 [frrouting](https://docs.frrouting.org/en/latest/bgp.html)

2. 集群节点配置 Calico BGP

 Calico 需要设置 `calico_backend: bird`, 否则无法建立 BGP 会话:

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
   
 本例因为节点的默认路由在网关, 节点之间不需要相互同步路由。所以关闭 Calico Full-Mesh:
 
   ```shell
   [root@master1 ~]# calicoctl patch bgpconfiguration default -p '{"spec": {"nodeToNodeMeshEnabled": false}}'
   ```

 创建 BGPPeer 资源:
 
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

3. 创建 Calico IPPool

我们需要为 SpiderPool 子网创建对应的 Calico IP 池, 否则 Calico 将不会宣告 SpiderPool 子网的路由:

   ```shell
   [root@master1 ~]# cat << EOF | calicoctl apply -f -
   apiVersion: projectcalico.org/v3
   kind: IPPool
   metadata:
     name: spiderpool-ippool
   spec:
     blockSize: 26
     cidr: 10.244.0.0/16
     ipipMode: Never
     natOutgoing: true
     nodeSelector: all()
     vxlanMode: Never
   ```

> cidr 需要对应 SpiderPool 的子网: `10.244.0.0/16`
> 禁用 IPIP 和 Vxlan 封装

4. 修改 Calico 的 `IPAM` 为 SpiderPool

修改每个节点上 Calico 的 CNI 配置文件: `/etc/cni/net.d/10-calico.conflist`, 将 ipam 字段切换为 SpiderPool:

```json
      "ipam": {                           "ipam": {
        "type": "calico-ipam"    ==>        "type": "spiderpool"
      },                                  },
```

如果有 `jq` 工具, 在每个节点执行以下命令:

```shell
cat <<< $(jq '.plugins[0].ipam.type = "spiderpool" ' /etc/cni/net.d/10-calico.conflist)
```

## 验证

1. 创建 Pod 验证 IP 分配情况

```shell
 cat <<EOF | kubectl create -f -
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   name: spiderpool-deploy
 spec:
   replicas: 2
   selector:
     matchLabels:
       app: spiderpool-deploy
   template:
     metadata:
       annotations:
        ipam.spidernet.io/ippool: '{"ipv4":["default-v4-ippool"]}'
       labels:
         app: spiderpool-deploy
     spec:
       containers:
       - name: spiderpool-deploy
         image: ghcr.io/daocloud/dao-2048:v1.2.0
         imagePullPolicy: IfNotPresent
         ports:
         - name: http
           containerPort: 80
           protocol: TCP
```

等待 Pod Running:

```shell
[root@master1 ~]# kubectl get po -o wide
NAME                                 READY   STATUS        RESTARTS   AGE     IP              NODE      NOMINATED NODE   READINESS GATES
spiderpool-deploy-644659db67-6lsmm   1/1     Running       0          12s     10.244.100.93   worker5   <none>           <none>
spiderpool-deploy-644659db67-n7ttd   1/1     Running       0          12s     10.244.100.71   master1   <none>           <none>
```

查看 IP 分配情况, 可以发现 SpiderPool 已经成功分配 IP: 

```shell
[root@master1 ~]# kubectl get sp default-ipv4-ippool -o yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"spiderpool.spidernet.io/v1","kind":"SpiderIPPool","metadata":{"annotations":{},"name":"default-ipv4-ippool"},"spec":{"ipVersion":4,"ips":["10.244.100.10-10.244.100.200"],"subnet":"10.244.0.0/16"}}
  creationTimestamp: "2023-03-15T02:12:00Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 1
  name: default-ipv4-ippool
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderSubnet
    name: default-ipv4-subnet
    uid: cd1598b7-cd80-4be3-a964-028eeb9c415d
  resourceVersion: "4788867"
  uid: 75881bdc-9930-4c76-b848-5af0f9188722
spec:
  default: false
  disable: false
  ipVersion: 4
  ips:
  - 10.244.100.10-10.244.100.200
  subnet: 10.244.0.0/16
  vlan: 0
status:
  allocatedIPCount: 2
  allocatedIPs: '{"10.244.100.71":{"interface":"eth0","namespace":"default","pod":"dao2048-calico-644659db67-n7ttd","uid":"65262cc2-6ff2-4121-80fa-0d21cc126367"},"10.244.100.93":{"interface":"eth0","namespace":"default","pod":"dao2048-calico-644659db67-6lsmm","uid":"7dd11d2a-1784-4795-8aa3-208e22ebb968"}}'
  totalIPCount: 191
```

2. 查看 Calico 是否成功宣告 BGP 路由:

```shell
root@router:~# vtysh

Hello, this is FRRouting (version 8.1).
Copyright 1996-2005 Kunihiro Ishiguro, et al.

router# show ip route bgp
Codes: K - kernel route, C - connected, S - static, R - RIP,
       O - OSPF, I - IS-IS, B - BGP, E - EIGRP, N - NHRP,
       T - Table, v - VNC, V - VNC-Direct, A - Babel, F - PBR,
       f - OpenFabric,
       > - selected route, * - FIB route, q - queued, r - rejected, b - backup
       t - trapped, o - offload failure

B>* 10.244.100.71/32 [20/0] via 172.16.1.11, eth1, weight 1, 00:13:08
B>* 10.244.100.93/32 [20/0] via 172.16.1.21, eth1, weight 1, 00:13:08
B>* 172.25.42.64/26 [20/0] via 172.16.1.11, eth1, weight 1, 1d19h40m
  *                        via 172.16.1.21, eth1, weight 1, 1d19h40m
B>* 172.25.137.64/26 [20/0] via 172.16.1.11, eth1, weight 1, 1d19h40m
  *                         via 172.16.1.21, eth1, weight 1, 1d19h40m
B>* 172.25.137.128/26 [20/0] via 172.16.1.11, eth1, weight 1, 1d19h40m
  *                          via 172.16.1.21, eth1, weight 1, 1d19h40m
B>* 172.25.199.128/26 [20/0] via 172.16.1.11, eth1, weight 1, 1d19h40m
  *                          via 172.16.1.21, eth1, weight 1, 1d19h40m
```

可以看到, Calico 已经将 Pod 路由同步到 BGP Router上了, 其中 32 位掩码的路由是来自从 SpiderPool (`10.244.0.0/16`) 子网分配 IP 的 Pod 路由; 26 位掩码是来自从 Calico 默认子网(`172.25.0.0/16`)分配 IP 的 Pod 路由。

3. 网络连通性测试

   - 集群外访问 Pod
   
   ```shell
   router# ping 10.244.100.71
   PING 10.244.100.71 (10.244.100.71) 56(84) bytes of data.
   64 bytes from 10.244.100.71: icmp_seq=1 ttl=63 time=0.473 ms
   64 bytes from 10.244.100.71: icmp_seq=2 ttl=63 time=0.903 ms
   --- 10.244.100.71 ping statistics ---
   2 packets transmitted, 2 received, 0% packet loss, time 1027ms
   rtt min/avg/max/mdev = 0.473/0.688/0.903/0.215 ms
   router# ping 10.244.100.93
   PING 10.244.100.93 (10.244.100.93) 56(84) bytes of data.
   64 bytes from 10.244.100.93: icmp_seq=1 ttl=63 time=0.669 ms
   64 bytes from 10.244.100.93: icmp_seq=2 ttl=63 time=0.655 ms
   --- 10.244.100.93 ping statistics ---
   2 packets transmitted, 2 received, 0% packet loss, time 1000ms
   rtt min/avg/max/mdev = 0.655/0.662/0.669/0.007 ms
   ```
   - Pod 跨节点访问

   ```shell
   [root@master1 ~]# kubectl exec spiderpool-deploy-644659db67-6lsmm -- ping -c2 10.244.100.71
   PING 10.244.100.71 (10.244.100.71): 56 data bytes
   64 bytes from 10.244.100.71: seq=0 ttl=61 time=1.172 ms
   64 bytes from 10.244.100.71: seq=1 ttl=61 time=1.674 ms
   
   --- 10.244.100.71 ping statistics ---
   2 packets transmitted, 2 packets received, 0% packet loss
   round-trip min/avg/max = 1.172/1.423/1.674 ms
   ```
   
   - Pod 跨节点访问 Calico IPPool 的 Pod:

   ```shell
   [root@master1 ~]# kubectl get po -n kube-system -o wide | grep coredns
   coredns-9ffd646b7-hhw77                    1/1     Running   0             3d      172.25.137.161   master1   <none>           <none>
   coredns-9ffd646b7-pnwzk                    1/1     Running   0             6d      172.25.42.75     worker5   <none>           <none>
   [root@master1 ~]# kubectl exec dao2048-calico-644659db67-6lsmm -- ping -c2 172.25.137.161
   PING 172.25.137.161 (172.25.137.161): 56 data bytes
   64 bytes from 172.25.137.161: seq=0 ttl=61 time=2.155 ms
   64 bytes from 172.25.137.161: seq=1 ttl=61 time=1.704 ms
   
   --- 172.25.137.161 ping statistics ---
   2 packets transmitted, 2 packets received, 0% packet loss
   round-trip min/avg/max = 1.704/1.929/2.155 ms
   ```
