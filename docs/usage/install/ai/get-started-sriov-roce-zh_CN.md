# AI Cluster With SR-IOV(RoCE)

**⚠️ 操作以下步骤之前，请确保您的环境已经达到 [环境要求](./index-zh_CN.md#环境要求)，并且按照 [主机准备](./index-zh_CN.md#主机准备) 完成 RoCE RDMA 模式下的主机配置。**

## 配置 SR-IOV operator

使用如下命令，查询主机上网卡设备的 PCIE 信息。确认如下输出的设备号 [15b3:1017] 出现在 [sriov-network-operator 支持网卡型号范围](https://github.com/k8snetworkplumbingwg/sriov-network-operator/blob/master/deployment/sriov-network-operator-chart/templates/configmap.yaml)

```shell
$ lspci -nn | grep Mellanox
    86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
    86:00.1 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
    ....
```

SRIOV VF 数量决定了一个网卡能同时为多少个 POD 提供网卡，不同型号的网卡的有不同的最大 VF 数量上限，Mellanox 的 ConnectX 网卡常见型号的最大 VF 上限是 127 。
如下示例，设置每个节点上的 GPU1 和 GPU2 的网卡，每个网卡配置出 12 个 VF 设备。请参考如下，为主机上每个亲和 GPU 的网卡配置 SriovNetworkNodePolicy，这样，将有 8 个 SRIOV resource 以供使用。

以下用 eno3np2 为例：

```shell
$ LINK_TYPE=eth NIC_NAME=eno3np2 VF_NUM=12
$ cat <<EOF | kubectl apply -f -
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetworkNodePolicy
metadata:
  name: ${NIC_NAME}
  namespace: spiderpool
spec:
  nodeSelector:
    kubernetes.io/os: "linux"
  resourceName: ${NIC_NAME}
  priority: 99
  numVfs: ${VF_NUM}
  nicSelector:
    pfNames:
      - ${NIC_NAME}
  linkType: ${LINK_TYPE}
  deviceType: netdevice
  isRdma: true
EOF
```

创建 SriovNetworkNodePolicy 配置后，每个节点上将会启动 sriov-device-plugin ，负责上报 VF 设备资源

```shell
$ kubectl get pod -n spiderpool
    operator-webhook-sgkxp                         1/1     Running     0          2m
    spiderpool-agent-9sllh                         1/1     Running     0          2m
    spiderpool-agent-h92bv                         1/1     Running     0          2m
    spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          2m
    spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          2m
    spiderpool-init                                0/1     Completed   0          2m
    sriov-device-plugin-x2g6b                      1/1     Running     0          1m
    sriov-device-plugin-z4gjt                      1/1     Running     0          1m
    sriov-network-config-daemon-8h576              1/1     Running     0          1m
    sriov-network-config-daemon-n629x              1/1     Running     0          1m
    .......
```

创建 SriovNetworkNodePolicy 配置后，SR-IOV operator 会顺序地在每一个节点上驱逐 POD，配置网卡驱动中的 VF 设置，然后重启主机。因此，会观测到集群中的节点会顺序进入 SchedulingDisabled 状态，并被重启。

```shell
$ kubectl get node
    NAME           STATUS                     ROLES                  AGE     VERSION
    ai-10-1-16-1   Ready                      worker                 2d15h   v1.28.9
    ai-10-1-16-2   Ready,SchedulingDisabled   worker                 2d15h   v1.28.9
    .......
```

所有节点完成 VF 配置的过程，可能需要数分钟，可以观察 sriovnetworknodestates 中的 status 是否进入 Succeeded 状态，表示配置完成

```shell
$ kubectl get sriovnetworknodestates -A
    NAMESPACE        NAME           SYNC STATUS   DESIRED SYNC STATE   CURRENT SYNC STATE   AGE
    spiderpool       ai-10-1-16-1   Succeeded     Idle                 Idle                 4d6h
    spiderpool       ai-10-1-16-2   Succeeded     Idle                 Idle                 4d6h
    .......
```

对于配置成功的节点，可查看 node 的可用资源，包含了上报的 SR-IOV 设备资源

```shell
$ kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
    [
      {
        "name": "ai-10-1-16-1",
        "allocable": {
          "cpu": "40",
          "pods": "110",
          "spidernet.io/eno3np2": "12",
          ...
        }
      },
      ...
    ]
```

<a id="create-spiderpool-resource"></a>

## 创建 CNI 配置和对应的 ippool 资源

### 共享子网方案配置

由于[共享子网组网方案](./index-zh_CN.md#共享子网组网方案roce)，所有节点的相同轨道网卡使用相同的子网，因此，只需要为每个节点的相同轨道网卡创建一个 IP 池即可.并且为所有的 GPU 亲和的 SR-IOV 网卡配置 [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) 配置，并创建对应的 IP 地址池 。 如下例子，配置了 GPU1 亲和的网卡: `eno3np2` 和 IP 地址池

```shell
$ cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np2
spec:
  gateway: 172.16.11.254
  subnet: 172.16.11.0/16
  ips:
    - 172.16.11.1-172.16.11.200
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: eno3np2
  namespace: spiderpool
spec:
  cniType: sriov
  sriov:
    resourceName: spidernet.io/eno3np2
    enableRdma: true
    ippools:
      ipv4: ["eno3np2"]
EOF
```

如果您需要自定义配置 VF 的 MTU，参考 [自定义配置 VF 的 MTU](#自定义-vf-的-mtu).

### 独享子网方案配置 

对于[独享子网组网方案](./index-zh_CN.md#独享子网组网方案roce) 下，每个节点的相同轨道网卡使用不同的子网，因此，需要为每个节点的每一个轨道网卡创建独立的 IP 池资源，这样 Spiderpool 可以保证当 Pod 调度到节点上时，能够基于 Pod 的节点 master 网卡所对应的 IP 池分配 IP。

创建 IP 池，为每个节点第一张 RDMA 网卡（一般是 1 号轨道网卡）并配置 RDMA 子网路由：

> 注意命名规则：建议 IP 池名称使用 `<interface>-rail<number>-<node>` 的格式，其中 `<interface>` 为网卡名称，`rail<number>` 为轨道名称， `<node>` 为节点名称， 这样 Spiderpool 可以自动根据节点和网卡名称识别并匹配 IP 池。

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np1-rail1-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.0/27
  gateway: 172.16.1.1
  ips:
    - 172.16.1.2-172.16.1.32
  routes:
    - to: 172.16.0.0/16
      via: 172.16.1.1
```

> routes 用于配置 RDMA 子网路由， 用于 Pod 访问 RDMA 子网的控制面通信。

依次为所有节点的每一轨的 RDMA 网卡创建 IP 池，，如节点 1 的其它 7 个轨道的 IP 池：

```shell
# Rail1 轨道的多个子网 IP 池
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np2-rail2-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.32/27
  gateway: 172.16.1.33
  ips:
    - 172.16.1.34-172.16.1.64
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np3-rail3-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.64/27
  gateway: 172.16.1.65
  ips:
    - 172.16.1.66-172.16.1.96
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np4-rail4-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.96/27
  gateway: 172.16.1.97
  ips:
    - 172.16.1.98-172.16.1.128
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np5-rail5-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.128/27
  gateway: 172.16.1.129
  ips:
    - 172.16.1.130-172.16.1.160
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np6-rail6-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.160/27
  gateway: 172.16.1.161
  ips:
    - 172.16.1.162-172.16.1.192
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np7-rail7-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.192/27
  gateway: 172.16.1.193
  ips:
    - 172.16.1.194-172.16.1.224
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: eno3np8-rail8-node1
spec:
  ipVersion: ipv4
  subnet: 172.16.1.224/27
  gateway: 172.16.1.225
  ips:
    - 172.16.1.226-172.16.1.256
EOF
```

> **注意**：为了简化示例，这里只展示了节点1 Rail1-Rail8 的配置。在实际环境中，需要为所有节点的每个 RDMA 网卡创建 IP 池。

创建支持子网自动匹配的 SpiderMultusConfig，以 eno3np1 网卡为例：

```shell
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: sriov-eno3np1
  namespace: kube-system
spec:
  cniType: sriov
  sriov:
    resourceName: "spidernet.io/eno3np1"
    rdmaIsolation: true
    mtu: 4200
    ippools:
      ipv4: 
      - eno3np1-rail1*
    matchMasterSubnet: true
EOF
```

**注意**：

> resourceName 需要匹配 sriovNodePolicy 中对应的 resourceName。
> 
> ippools.ipv4 使用了通配符 `eno3np1-rail1*`，分配 IP 时 Spiderpool 会从所有以 `eno3np1-rail1` 开头的 IP 池中筛选。 这里会得到所有节点的 1 号轨道网卡的 IP 池。
>
> matchMasterSubnet 设置为 true，当 Spiderpool 为 Pod 分配 IP 时，会判断已筛选子网池中是否存在匹配到当前 Pod 网卡对应的 Master 网卡的子网，如果存在则分配 IP，否则分配失败。

依次创建其它轨道的 SpiderMultusConfig，为 eno3np2 网卡为例：

```shell
# Rail2 轨道的自动子网匹配配置
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: sriov-eno3np2
  namespace: kube-system
spec:
  cniType: sriov
  sriov:
    resourceName: "spidernet.io/eno3np2"
    enableRdma: true
    mtu: 4200
    ippools:
      ipv4: 
      - eno3np2-rail2*
    matchMasterSubnet: true
EOF
```

## 创建测试应用

1. 在指定节点上创建一组 DaemonSet 应用，测试指定节点上的 SR-IOV 设备的可用性
    如下例子，通过 annotations `v1.multus-cni.io/default-network` 指定使用 calico 的缺省网卡，用于进行控制面通信，annotations `k8s.v1.cni.cncf.io/networks` 接入 8 个 GPU 亲和网卡的 VF 网卡，用于 RDMA 通信，并配置 8 种 RDMA resources 资源

    > 注：支持自动为应用注入 RDMA 网络资源，参考 [基于 Webhook 自动为应用注入 RDMA 网络资源](#基于-webhook-自动注入-rdma-网络资源)

    ```shell
    $ helm repo add spiderchart https://spidernet-io.github.io/charts
    $ helm repo update
    $ helm search repo rdma-tools
   
    # run daemonset on worker1 and worker2
    $ cat <<EOF > values.yaml
    # for china user , it could add these to use a domestic registry
    #image:
    #  registry: ghcr.m.daocloud.io

    # just run daemonset in nodes 'worker1' and 'worker2'
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/hostname
              operator: In
              values:
              - worker1
              - worker2

    # sriov interfaces
    extraAnnotations:
      k8s.v1.cni.cncf.io/networks: |-
        [{"name":"gpu1-sriov","namespace":"spiderpool"},
        {"name":"gpu2-sriov","namespace":"spiderpool"},
        {"name":"gpu3-sriov","namespace":"spiderpool"},
        {"name":"gpu4-sriov","namespace":"spiderpool"},
        {"name":"gpu5-sriov","namespace":"spiderpool"},
        {"name":"gpu6-sriov","namespace":"spiderpool"},
        {"name":"gpu7-sriov","namespace":"spiderpool"},
        {"name":"gpu8-sriov","namespace":"spiderpool"}]

    # sriov resource
    resources:
      limits:
        spidernet.io/gpu1sriov: 1
        spidernet.io/gpu2sriov: 1
        spidernet.io/gpu3sriov: 1
        spidernet.io/gpu4sriov: 1
        spidernet.io/gpu5sriov: 1
        spidernet.io/gpu6sriov: 1
        spidernet.io/gpu7sriov: 1
        spidernet.io/gpu8sriov: 1
        #nvidia.com/gpu: 1
    EOF

    $ helm install rdma-tools spiderchart/rdma-tools -f ./values.yaml
    
    ```

    在容器的网络命名空间创建过程中，Spiderpool 会对 sriov 接口上的网关进行连通性测试，如果如上应用的所有 POD 都启动成功，说明了每个节点上的 VF 设备的连通性成功，可进行正常的 RDMA 通信。

    <a id="checking-pod-network"></a>

2. 查看容器的网络命名空间状态

    可进入任一一个 POD 的网络命名空间中，确认具备 9 个网卡, 对于[独享子网方案](./index-zh_CN.md#独享子网组网方案roce)场景下，需要额外检查 Pod net1-net8 的 IP 地址是否属于该节点的 eno3np1-eno3np8 的 IP 地址范围，并且检查 main 路由表中是否具备 RDMA 子网路由：

    ```shell
    root@rdma-tools-4v8t8:/# ip r show table main
    ...
    172.16.0.0/16 via 172.16.1.1 dev net1
    ...
    ```

    ```shell
    $ kubectl exec -it rdma-tools-4v8t8  bash
    kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
    root@rdma-tools-4v8t8:/# ip a
       1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
           link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
           inet 127.0.0.1/8 scope host lo
              valid_lft forever preferred_lft forever
           inet6 ::1/128 scope host
              valid_lft forever preferred_lft forever
       2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN group default qlen 1000
           link/ipip 0.0.0.0 brd 0.0.0.0
       3: eth0@if356: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1480 qdisc noqueue state UP group default qlen 1000
           link/ether ca:39:52:fc:61:cd brd ff:ff:ff:ff:ff:ff link-netnsid 0
           inet 10.233.119.164/32 scope global eth0
              valid_lft forever preferred_lft forever
           inet6 fe80::c839:52ff:fefc:61cd/64 scope link
              valid_lft forever preferred_lft forever
       269: net1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP group default qlen 1000
           link/ether 3a:97:49:35:79:95 brd ff:ff:ff:ff:ff:ff
           inet 172.16.11.10/24 brd 10.1.19.255 scope global net1
              valid_lft forever preferred_lft forever
           inet6 fe80::3897:49ff:fe35:7995/64 scope link
              valid_lft forever preferred_lft forever
       239: net2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP group default qlen 1000
           link/ether 1e:b6:13:0e:2a:d5 brd ff:ff:ff:ff:ff:ff
           inet 172.16.12.10/24 brd 10.1.19.255 scope global net1
              valid_lft forever preferred_lft forever
           inet6 fe80::1cb6:13ff:fe0e:2ad5/64 scope link
              valid_lft forever preferred_lft forever
       .....
    ```

    查看路由配置，Spiderpool 会自动为每个网卡调谐策略路由，确保每个网卡上收到的外部请求都会从该网卡上返回回复流量

    ```shell
    root@rdma-tools-4v8t8:/# ip rule
    0:  from all lookup local
    32762:  from 172.16.11.10 lookup 107
    32763:  from 172.16.12.10 lookup 106
    32764:  from 172.16.13.10 lookup 105
    32765:  from 172.16.14.10 lookup 104
    32765:  from 172.16.15.10 lookup 103
    32765:  from 172.16.16.10 lookup 102
    32765:  from 172.16.17.10 lookup 101
    32765:  from 172.16.18.10 lookup 100
    32766:  from all lookup main
    32767:  from all lookup default

    root@rdma-tools-4v8t8:/# ip route show table 100
        default via 172.16.11.254 dev net1
    ```

    main 路由中，确保了 calico 网络流量、ClusterIP 流量、本地宿主机通信等流量都会从 calico 网卡转发

    ```shell
    root@rdma-tools-4v8t8:/# ip r show table main
        default via 169.254.1.1 dev eth0
        172.16.11.0/24 dev net1 proto kernel scope link src 172.16.11.10
        172.16.12.0/24 dev net2 proto kernel scope link src 172.16.12.10
        172.16.13.0/24 dev net3 proto kernel scope link src 172.16.13.10
        172.16.14.0/24 dev net4 proto kernel scope link src 172.16.14.10
        172.16.15.0/24 dev net5 proto kernel scope link src 172.16.15.10
        172.16.16.0/24 dev net6 proto kernel scope link src 172.16.16.10
        172.16.17.0/24 dev net7 proto kernel scope link src 172.16.17.10
        172.16.18.0/24 dev net8 proto kernel scope link src 172.16.18.10
        10.233.0.0/18 via 10.1.20.4 dev eth0 src 10.233.119.164
        10.233.64.0/18 via 10.1.20.4 dev eth0 src 10.233.119.164
        10.233.119.128 dev eth0 scope link src 10.233.119.164
        169.254.0.0/16 via 10.1.20.4 dev eth0 src 10.233.119.164
        169.254.1.1 dev eth0 scope link
    ```

    确认具备 8 个 RDMA 设备

    ```shell
    root@rdma-tools-4v8t8:/# rdma link
        link mlx5_27/1 state ACTIVE physical_state LINK_UP netdev net2
        link mlx5_54/1 state ACTIVE physical_state LINK_UP netdev net1
        link mlx5_67/1 state ACTIVE physical_state LINK_UP netdev net4
        link mlx5_98/1 state ACTIVE physical_state LINK_UP netdev net3
        .....
    ```

3. 在跨节点的 Pod 之间，确认 RDMA 收发数据正常

    开启一个终端，进入一个 Pod 启动服务

    ```shell
    # see 8 RDMA devices assigned to the Pod
    $ rdma link

    # Start an RDMA service
    $ ib_read_lat
    ```

    开启一个终端，进入另一个 Pod 访问服务：

    ```shell
    # You should be able to see all RDMA network cards on the host
    $ rdma link
        
    # Successfully access the RDMA service of the other Pod
    $ ib_read_lat 172.91.0.115
    ```

    观察 RDMA 流量统计可通过进入到容器执行 `rdma statistic` 或参考 [RDMA监控](../../rdma-metrics-zh_CN.md).

## 基于 Webhook 自动注入 RDMA 网络资源

在上述步骤中，我们展示了如何使用 SR-IOV 技术在 RoCE 和 Infiniband 网络环境中为容器提供 RDMA 通信能力。然而，当配置多网卡的 AI 应用时，过程会变得复杂。为简化这个过程，Spiderpool 通过 annotations(`cni.spidernet.io/rdma-resource-inject` 或 `cni.spidernet.io/network-resource-inject`) 支持对一组网卡配置进行分类。用户只需要为应用添加与网卡配置相同的注解，Spiderpool 就会通过 webhook 自动为应用注入所有具有相同注解的对应网卡和网络资源。`cni.spidernet.io/rdma-resource-inject` 只适用于 AI 场景，自动注入 RDMA 网卡及 RDMA Resources；`cni.spidernet.io/network-resource-inject` 不但可以用于 AI 场景，也支持 Underlay 场景。在未来我们希望都统一使用 `cni.spidernet.io/network-resource-inject` 支持这两种场景。

  > 该功能仅支持 [ macvlan, ipvlan, sriov, ib-sriov, ipoib ] 这几种 cniType 的网卡配置。

1. 当前 Spiderpool 的 webhook 自动注入 RDMA 网络资源，默认是关闭的，需要手动开启。

    ```shell
    ~# helm upgrade --install spiderpool spiderpool/spiderpool --namespace spiderpool --create-namespace --reuse-values --set spiderpoolController.podResourceInject.enabled=true
    ```

    > 启用 webhook 自动注入网络资源功能后，您可以通过更新 configMap: spiderpool-config 中的 podResourceInject 字段更新配置。
    >
    > 通过 `podResourceInject.namespacesExclude` 指定不进行 RDMA 网络资源注入的命名空间
    >
    > 通过 `podResourceInject.namespacesInclude` 指定需要进行 RDMA 网络资源注入的命名空间，如果 `podResourceInject.namespacesExclude` 和 `podResourceInject.namespacesInclude` 都没有指定，则默认对所有命名空间进行 RDMA 网络资源注入。
    >
    > 当前，完成配置变更后，您需要重启 spiderpool-controller 来使配置生效。

2. 在创建 AI 算力网络的所有 SpiderMultusConfig 实例时，添加 key 为 "cni.spidernet.io/rdma-resource-inject" 或 "cni.spidernet.io/network-resource-inject" 的 annotation，value 可自定义任何值
  
    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: gpu1-net11
    spec:
      gateway: 172.16.11.254
      subnet: 172.16.11.0/16
      ips:
      - 172.16.11.1-172.16.11.200
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: gpu1-sriov
      namespace: spiderpool
      annotations:
        cni.spidernet.io/rdma-resource-inject: rdma-network
    spec:
      cniType: sriov
      sriov:
        resourceName: spidernet.io/gpu1sriov
        enableRdma: true
      ippools:
        ipv4: ["gpu1-net11"]
    ```

3. 创建 AI 应用时，为应用也添加相同注解：

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
            cni.spidernet.io/rdma-resource-inject: rdma-network
    ```

    > 注意：使用 webhook 自动注入网络资源功能时，不能为应用添加其他网络配置注解(如 `k8s.v1.cni.cncf.io/networks` 和 `ipam.spidernet.io ippools`等)，Spiderpool 注入时会清理这些注解，否则会影响资源自动注入功能。

4. 当 Pod 被创建后，可观测到 Pod 被自动注入了网卡 annotation 和 RDMA 资源

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
              k8s.v1.cni.cncf.io/networks: |-
                [{"name":"gpu1-sriov","namespace":"spiderpool"},
                {"name":"gpu2-sriov","namespace":"spiderpool"},
                {"name":"gpu3-sriov","namespace":"spiderpool"},
                {"name":"gpu4-sriov","namespace":"spiderpool"},
                {"name":"gpu5-sriov","namespace":"spiderpool"},
                {"name":"gpu6-sriov","namespace":"spiderpool"},
                {"name":"gpu7-sriov","namespace":"spiderpool"},
                {"name":"gpu8-sriov","namespace":"spiderpool"}]
         ....
         resources:
           limits:
             spidernet.io/gpu1rdma: 1
             spidernet.io/gpu2rdma: 1
             spidernet.io/gpu3rdma: 1
             spidernet.io/gpu4rdma: 1
             spidernet.io/gpu5rdma: 1
             spidernet.io/gpu6rdma: 1
             spidernet.io/gpu7rdma: 1
             spidernet.io/gpu8rdma: 1
    ```

## 自定义 VF 的 MTU

  默认情况下，SR-IOV VF 的 MTU 不会继承其 PF 的值影响，因此在一些特殊通信场景下，用户需要为 Pod 自定义 MTU 大小以满足不同数据报文通信需求。您可以参考以下方式自定义配置 Pod 的 MTU 大小(以 Ethernet 为例)：

  ```yaml
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderMultusConfig
  metadata:
    name: gpu1-sriov
    namespace: spiderpool
  spec:
    cniType: sriov
    sriov:
      resourceName: spidernet.io/gpu1sriov
      enableRdma: true
      mtu: 8000
      ippools:
        ipv4: ["gpu1-net11"]
  ```

  注意：MTU 的取值范围不应该大于 sriov PF 的 MTU 值。
