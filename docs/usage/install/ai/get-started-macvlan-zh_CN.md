# AI Cluster With Macvlan(RoCE)

**⚠️ 操作以下步骤之前，请确保您的环境已经达到 [环境要求](./index-zh_CN.md#环境要求)，并且按照 [主机准备](./index-zh_CN.md#主机准备) 完成共享 RDMA 模式下的主机配置。**

## 配置 k8s-rdma-shared-dev-plugin

首先需要配置 k8s-rdma-shared-dev-plugin, 以识别出每个主机上的 RDMA 共享设备资源并通告给 kubelet:

修改如下 configmap，创建出 8 种 RDMA 共享设备，它们分别亲和每一个 GPU 设备。configmap 的详细配置可参考[官方文档](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin?tab=readme-ov-file#rdma-shared-device-plugin-configurations)。

```shell
$ kubectl edit configmap -n spiderpool spiderpool-rdma-shared-device-plugi
  ....
  config.json: |
    {
      "periodicUpdateInterval": 300,
      "configList": [
        {
          "resourcePrefix": "spidernet.io",
          "resourceName": "shared_cx5_gpu1",
          "rdmaHcaMax": 100,
          "selectors": { "ifNames": ["enp11s0f0np0"] }
        },
        ....
        {
          "resourcePrefix": "spidernet.io",
          "resourceName": "shared_cx5_gpu8",
          "rdmaHcaMax": 100,
          "selectors": { "ifNames": ["enp18s0f0np0"] }
        }
      ]
```

完成如上配置后，可查看 node 的可用资源，确认每个节点都正确识别并上报了 8 种 RDMA 设备资源。

```shell
$ kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
    [
      {
        "name": "ai-10-1-16-1",
        "allocable": {
          "cpu": "40",
          "pods": "110",
          "spidernet.io/shared_cx5_gpu1": "100",
          "spidernet.io/shared_cx5_gpu2": "100",
          ...
          "spidernet.io/shared_cx5_gpu8": "100",
          ...
        }
      },
      ...
    ]
```

<a id="create-spiderpool-resource"></a>

## 创建 Spiderpool 资源

### 创建 CNI 配置和对应的 ippool 资源


## 创建测试应用

1. 在指定节点上创建一组 DaemonSet 应用

    如下例子，通过 annotations `v1.multus-cni.io/default-network` 指定使用 calico 的缺省网卡，用于进行控制面通信，annotations `k8s.v1.cni.cncf.io/networks` 接入 8 个 GPU 亲和网卡的网卡，用于 RDMA 通信，并配置 8 种 RDMA resources 资源

    > 注：可自动为应用注入 RDMA 网络资源，参考 [基于 Webhook 自动注入 RDMA 资源](#基于-webhook-自动注入-rdma-网络资源)

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

    # macvlan interfaces
    extraAnnotations:
      k8s.v1.cni.cncf.io/networks: |-
        [{"name":"gpu1-macvlan","namespace":"spiderpool"},
        {"name":"gpu2-macvlan","namespace":"spiderpool"},
        {"name":"gpu3-macvlan","namespace":"spiderpool"},
        {"name":"gpu4-macvlan","namespace":"spiderpool"},
        {"name":"gpu5-macvlan","namespace":"spiderpool"},
        {"name":"gpu6-macvlan","namespace":"spiderpool"},
        {"name":"gpu7-macvlan","namespace":"spiderpool"},
        {"name":"gpu8-macvlan","namespace":"spiderpool"}]

    # macvlan resource
    resources:
      limits:
        spidernet.io/shared_cx5_gpu1: 1
        spidernet.io/shared_cx5_gpu2: 1
        spidernet.io/shared_cx5_gpu3: 1
        spidernet.io/shared_cx5_gpu4: 1
        spidernet.io/shared_cx5_gpu5: 1
        spidernet.io/shared_cx5_gpu6: 1
        spidernet.io/shared_cx5_gpu7: 1
        spidernet.io/shared_cx5_gpu8: 1
        #nvidia.com/gpu: 1
    EOF

    $ helm install rdma-tools spiderchart/rdma-tools -f ./values.yaml
    ```

    在容器的网络命名空间创建过程中，Spiderpool 会对 macvlan 接口上的网关进行连通性测试，如果如上应用的所有 POD 都启动成功，说明了每个节点上的 VF 设备的连通性成功，可进行正常的 RDMA 通信。

    <a id="checking-pod-network"></a>

2. 查看容器的网络命名空间状态

    可进入任一一个 POD 的网络命名空间中，确认具备 9 个网卡：

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

    查看路由配置，Spiderpool 会自动为每个网卡调谐策略路由，确保每个网卡上收到的外部请求都会从该网卡上返回回复流量：

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

    开启一个终端，进入一个 Pod 启动服务：

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
      cniType: macvlan
      macvlan:
        master: ["enp11s0f0np0"]
        enableRdma: true
        rdmaResourceName: spidernet.io/gpu1rdma
      ippools:
        ipv4: ["gpu1-net11"]
    ```

3. 创建 AI 应用时，为应用也添加相同注解:

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
            cni.spidernet.io/rdma-resource-inject: rdma-network
    ```

   > 注意：使用 webhook 自动注入网络资源功能时，不能为应用添加其他网络配置注解(如 `k8s.v1.cni.cncf.io/networks` 和 `ipam.spidernet.io ippools`等)，否则会影响资源自动注入功能。

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
