# Calico Quick Start

[**English**](./get-started-calico.md) | **简体中文**

本文将介绍在一个 [Calico](https://github.com/projectcalico/calico) 作为缺省 CNI 的集群，通过 `Spiderpool` 这一完整的 Underlay 网络解决方案，通过 Multus 为 Pod 额外附加一张由 `Macvlan` 创建的网卡，并通过 `coordinator` 解决 Pod 多网卡之间路由调协问题。该方案可实现 Pod 访问集群内东西向流量从 Calico 创建的网卡转发(eth0)， 它的好处是：

- 集群外部客户端可直接通过 Pod 的 Underlay IP 访问 Pod, 而无需借助 NodePort 的方式暴露 Pod。
- 可为 Pod 单独接入一张 Underlay 网卡，使 Pod 单独接入存储等专用网络，保障独立带宽。
- 当 Pod 附加了 Calico 和 Macvlan 多张网卡时，可通过路由调谐，基于 Calico 实现 Pod 的 Underlay IP 访问 ClusterIP 的问题。
- 当 Pod 附加了 Calico 和 Macvlan 多张网卡时，调谐 Pod 的子网路由，确保 Pod 数据包访问时的来回路径一致，避免路径问题而导致路由器丢包。
- 可基于 Pod 的 annotation: ipam.spidernet.io/default-route-nic 灵活指定 Pod 默认路由的所在网卡。

> 注: 本文将使用简写 NAD 来代指 Multus CRD NetworkAttachmentDefinition ，NAD 为其首字母简写

## 先决条件

- [安装要求](./../system-requirements-zh_CN.md)
- 准备好一个 Kubernetes 集群
- 安装 Calico 作为集群的缺省 CNI。如果未安装，可参考 [官方文档](https://docs.tigera.io/calico/latest/getting-started/kubernetes/) 或参考以下命令安装:

   ```shell
   ~# kubectl apply -f https://github.com/projectcalico/calico/blob/master/manifests/calico.yaml
   ~# kubectl wait --for=condition=ready -l k8s-app=calico-node  pod -n kube-system 
   ```

- Helm 二进制

## 安装 Spiderpool

使用以下命令安装 Spiderpool:

```shell
~# helm repo add spiderpool https://spidernet-io.github.io/spiderpool
~# helm repo update spiderpool
~# helm install spiderpool spiderpool/spiderpool --namespace kube-system  --set coordinator.mode=overlay --wait 
```

> 如果您的集群未安装 Macvlan CNI, 可指定 Helm 参数 `--set plugins.installCNI=true` 安装 Macvlan 等 CNI 到每个节点。
>
> 通过 `multus.multusCNI.defaultCniCRName` 指定 multus 默认使用的 CNI 的 NetworkAttachmentDefinition 实例名。如果 `multus.multusCNI.defaultCniCRName` 选项不为空，则安装后会自动生成一个数据为空的 NetworkAttachmentDefinition 对应实例。如果 `multus.multusCNI.defaultCniCRName` 选项为空，会尝试通过 /etc/cni/net.d 目录下的第一个 CNI 配置来创建对应的 NetworkAttachmentDefinition 实例，否则会自动生成一个名为 `default` 的 NetworkAttachmentDefinition 实例，以完成 multus 的安装。

等待安装完成，查看 Spiderpool 组件状态:

```shell
~# kubectl get po -n kube-system | grep spiderpool
spiderpool-agent-htcnc                                      1/1     Running     0                 1m
spiderpool-agent-pjqr9                                      1/1     Running     0                 1m
spiderpool-controller-7b7f8dd9cc-xdj95                      1/1     Running     0                 1m
spiderpool-init                                             0/1     Completed   0                 1m
```

请检查 `Spidercoordinator.status` 中的 Phase 是否为 Synced, 并且 overlayPodCIDR 是否与集群中 Calico 的子网保持一致:

```shell
~# calicoctl get ippools
NAME                   CIDR             SELECTOR
default-ipv4-ippool    10.222.64.0/18   all()
default-ipv4-ippool1   10.223.64.0/18   all()

~# kubectl  get spidercoordinators.spiderpool.spidernet.io default -o yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderCoordinator
metadata:
  creationTimestamp: "2023-10-18T08:31:09Z"
  finalizers:
  - spiderpool.spidernet.io
  generation: 7
  name: default
  resourceVersion: "195405"
  uid: 8bdceced-15db-497b-be07-81cbcba7caac
spec:
  detectGateway: false
  detectIPConflict: false
  hijackCIDR:
  - 169.254.0.0/16
  hostRPFilter: 0
  hostRuleTable: 500
  mode: auto
  podCIDRType: calico
  podDefaultRouteNIC: ""
  podMACPrefix: ""
  tunePodRoutes: true
status:
  overlayPodCIDR:
  - 10.222.64.0/18
  - 10.223.64.0/18
  phase: Synced
  serviceCIDR:
  - 10.233.0.0/18
```
 
> 目前 Spiderpool 优先通过查询 `kube-system/kubeadm-config` ConfigMap 获取集群的 Pod 和 Service 子网。 如果 kubeadm-config 不存在导致无法获取集群子网，那么 Spiderpool 会从 Kube-controller-manager Pod 中获取集群 Pod 和 Service 的子网。 如果您集群的 Kube-controller-manager 组件以 `systemd` 方式而不是以静态 Pod 运行。那么 Spiderpool 仍然无法获取集群的子网信息。

如果上面两种方式都失败，Spiderpool 会同步 status.phase 为 NotReady, 这将会阻止 Pod 被创建。我们可以通过下面解决异常情况:

- 手动创建 kubeadm-config ConfigMap, 并正确配置集群的子网信息:

```shell
export POD_SUBNET=<YOUR_POD_SUBNET>
export SERVICE_SUBNET=<YOUR_SERVICE_SUBNET>
cat << EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubeadm-config
  namespace: kube-system
data:
  ClusterConfiguration: 
    networking:
      podSubnet: ${POD_SUBNET}
      serviceSubnet: ${SERVICE_SUBNET}
EOF
```

一旦创建完成，Spiderpool 将会自动同步其状态。

### 创建 SpiderIPPool

本文集群节点网卡: `ens192` 所在子网为 `10.6.0.0/16`, 以该子网创建 SpiderIPPool:

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: 10-6-v4
spec:
  disable: false
  gateway: 10.6.0.1
  ips:
  - 10.6.212.100-10.6.212.200
  subnet: 10.6.0.0/16
EOF
```

> subnet 应该与节点网卡 `ens192` 的子网保持一致，并且 ips 不与现有任何 IP 冲突。

### 创建 SpiderMultusConfig

本文通过 Spidermultusconfig 创建 Multus 的 NAD 实例:

```shell
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: macvlan-ens192
spec:
  cniType: macvlan
  macvlan:
    master:
    - ens192
    ippools:
      ipv4:
      - 10-6-v4
    vlanID: 0
EOF
```

> `spec.macvlan.master` 设置为 `ens192`, `ens192`必须存在于主机上。并且 `spec.macvlan.spiderpoolConfigPools.IPv4IPPool`设置的子网和 `ens192`保持一致。

创建成功后, 查看 Multus NAD 是否成功创建:

```shell
~# kubectl  get network-attachment-definitions.k8s.cni.cncf.io  macvlan-ens192 -o yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"spiderpool.spidernet.io/v2beta1","kind":"SpiderMultusConfig","metadata":{"annotations":{},"name":"macvlan-ens192","namespace":"default"},"spec":{"cniType":"macvlan","coordinator":{"podCIDRType":"cluster","tuneMode":"overlay"},"enableCoordinator":true,"macvlan":{"master":["ens192"],"spiderpoolConfigPools":{"IPv4IPPool":["10-6-v4"]},"vlanID":0}}}
  creationTimestamp: "2023-06-30T07:12:21Z"
  generation: 1
  name: macvlan-ens192
  namespace: default
  ownerReferences:
  - apiVersion: spiderpool.spidernet.io/v2beta1
    blockOwnerDeletion: true
    controller: true
    kind: SpiderMultusConfig
    name: macvlan-ens192
    uid: 3f902f46-d9d4-4c62-a7c3-98d4a9aa26e4
  resourceVersion: "24713635"
  uid: 712d1e58-ab57-49a7-9189-0fffc64aa9c3
spec:
  config: '{"cniVersion":"0.3.1","name":"macvlan-ens192","plugins":[{"type":"macvlan","ipam":{"type":"spiderpool","default_ipv4_ippool":["10-6-v4"]},"master":"ens192","mode":"bridge"},{"type":"coordinattor","ipam":{},"dns":{},"detectGateway":false,"tunePodRoutes":true,"mode":"overlay","hostRuleTable":500,"detectIPConflict":false}]}'
```

### 创建应用

使用下面的命令创建测试应用 nginx:

```shell
~# cat <<EOF | kubectl create -f -
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
        k8s.v1.cni.cncf.io/networks: macvlan-ens192
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

- `k8s.v1.cni.cncf.io/networks`: 该字段指定 Multus 使用 `macvlan-ens192` 为 Pod 附加一张网卡。

等待 Pod ready, 查看 IP 分配情况:

```shell
~#  kubectl get po -l app=nginx -o wide
NAME                     READY   STATUS    RESTARTS   AGE   IP                NODE        NOMINATED NODE   READINESS GATES
nginx-4653bc4f24-aswpm   1/1     Running   0          2m    10.233.105.167    controller  <none>           <none>
nginx-4653bc4f24-rswak   1/1     Running   0          2m    10.233.73.210     worker01    <none>           <none>
```

```shell
~# kubectl get se
NAME                     INTERFACE   IPV4POOL            IPV4               IPV6POOL   IPV6   NODE
nginx-4653bc4f24-rswak   net1        10-6-v4             10.6.212.145/16                      worker01
nginx-4653bc4f24-aswpm   net1        10-6-v4             10.6.212.148/16                      controller
```

进入到 Pod 内部， 通过 `ip` 命令查看 Pod 中 IP、路由等信息:

```shell
[root@controller1 ~]# kubectl exec it nginx-4653bc4f24-rswak sh
# ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN group default qlen 1000
    link/ipip 0.0.0.0 brd 0.0.0.0
4: eth0@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1430 qdisc noqueue state UP group default
    link/ether a2:99:9d:04:01:80 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.233.73.210/32 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fd85:ee78:d8a6:8607::1:eb84/128 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::a099:9dff:fe04:180/64 scope link
       valid_lft forever preferred_lft forever
5: net1@if2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 2a:1e:a1:db:2a:9a brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.6.212.145/16 brd 10.6.255.255 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fd00:10:6::2e5/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::281e:a1ff:fedb:2a9a/64 scope link
       valid_lft forever preferred_lft forever
/# ip rule
0: from all lookup local
32760: from 10.6.212.132 lookup 100
32766: from all lookup main
32767: from all lookup default
/# ip route
default via 169.254.1.1 dev eth0
10.6.0.0/16 dev net1 scope link  src 10.6.212.145
10.6.212.132 dev eth0 scope link
10.233.0.0/18 via 10.6.212.132 dev eth0 
10.233.64.0/18 via 10.6.212.132 dev eth0
169.254.1.1 dev eth0 scope link
/ # ip route show table 100
default via 10.6.0.1 dev net1
10.6.0.0/16 dev net1 scope link  src 10.6.212.145
10.6.212.132 dev eth0 scope link
10.233.0.0/18 via 10.6.212.132 dev eth0 
10.233.64.0/18 via 10.6.212.132 dev eth0
```

以上表项解释:

> Pod 中分配了 Calico(eth0) 和 Macvlan(net1) 两张网卡, IPv4 地址分别是: 10.233.73.210 和 10.6.212.145
>
> 10.233.0.0/18 和 10.233.64.0/18 是集群的 CIDR, Pod访问该子网时从 eth0 转发, 每个 route table 都会插入此路由
>
> 10.6.212.132 是 Pod 所在节点的地址，此路由确保 Pod 访问该主机时从 eth0 转发
>
> 这一系列的路由确保 Pod 访问集群内目标时从 eth0 转发，访问外部目标时从 net1 转发
>
> 在默认情况下，Pod 的默认路由保留在 eth0。如果想要保留在其他网卡(如 net1)，可以通过在 Pod 的 annotations 中注入: "ipam.spidernet.io/default-route-nic: net1" 实现。

下面测试 Pod 基本网络连通性，以访问 CoreDNS 的 Pod 和 Service 为例:

```shell
~# kubectl  get all -n kube-system -l k8s-app=kube-dns -o wide
NAME                           READY   STATUS    RESTARTS      AGE   IP               NODE          NOMINATED NODE   READINESS GATES
pod/coredns-57fbf68cf6-2z65h   1/1     Running   1 (91d ago)   91d   10.233.105.131   worker1       <none>           <none>
pod/coredns-57fbf68cf6-kvcwl   1/1     Running   3 (91d ago)   91d   10.233.73.195    controller    <none>           <none>

NAME              TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)                  AGE   SELECTOR
service/coredns   ClusterIP   10.233.0.3   <none>        53/UDP,53/TCP,9153/TCP   91d   k8s-app=kube-dns

~# 跨节点访问 CoreDNS 的 Pod
~# kubectl  exec nginx-4653bc4f24-rswak -- ping 10.233.73.195 -c 2
PING 10.233.73.195 (10.233.73.195): 56 data bytes
64 bytes from 10.233.73.195: seq=0 ttl=62 time=2.348 ms
64 bytes from 10.233.73.195: seq=1 ttl=62 time=0.586 ms

--- 10.233.73.195 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.586/1.467/2.348 ms

~# 访问 CoreDNS 的 service
~# kubectl exec  nginx-4653bc4f24-rswak -- curl 10.233.0.3:53 -I
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:--  0:00:02 --:--:--     0
curl: (52) Empty reply from server
```

测试 Pod 访问集群南北向流量的联通性，以访问其他网段目标(10.7.212.101)为例:

```shell
[root@controller1 cyclinder]# kubectl exec nginx-4653bc4f24-rswak -- ping 10.7.212.101 -c 2
PING 10.7.212.101 (10.7.212.101): 56 data bytes
64 bytes from 10.7.212.101: seq=0 ttl=61 time=4.349 ms
64 bytes from 10.7.212.101: seq=1 ttl=61 time=0.877 ms

--- 10.7.212.101 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.877/2.613/4.349 ms
```
