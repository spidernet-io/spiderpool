# RDMA with RoCE

**简体中文** | [**English**](./rdma-roce.md)

## 介绍

本节介绍基于主机上的 RoCE 网卡，如何给 POD 分配 RDMA 网卡。

## 功能

RDMA 设备的网络命名空间具备 shared 和 exclusive 两种模式，容器因此可以实现共享 RDMA 网卡，或者独享 RDMA 网卡。在 kubernetes 下，可基于 macvlan 或 ipvlan CNI 来使用 shared 模式的 RoCE
RDMA 网卡，也可以基于 SR-IOV CNI 来使用 exclusive 模式的网卡。

- 在 shared 模式下，Spiderpool 使用了 macvlan 或 ipvlan CNI 来暴露宿主机上的 RoCE 网卡给 Pod 使用，使用 [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin) 来完成 RDMA 网卡资源的暴露和 Pod 调度。

- 在 exclusive 模式下，Spiderpool 使用了 [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-network-operator) 来暴露宿主机上的 RDMA 网卡给 Pod 使用，暴露 RDMA 资源。使用 [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni) 来完成 RDMA 设备隔离。

    对于隔离 RDMA 网卡，必须至少满足以下条件之一：

    （1） 基于 5.3.0 或更新版本的 Linux 内核，系统中加载的RDMA模块，rdma核心包提供了在系统启动时自动加载相关模块的方法

    （2） 需要 Mellanox OFED 4.7 版或更新版本。在这种情况下，不需要使用基于 5.3.0 或更新版本的内核。

## 基于 macvlan 或 ipvlan 共享使用 RDMA 网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 macvlan CNI 使得 Pod 共享使用 RDMA 设备：

1. 在宿主机上，确保主机拥有 RDMA 网卡，且安装好驱动，确保 RDMA 功能工作正常。

    本示例环境中，宿主机上具备 RoCE 功能的 mellanox ConnectX 5 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。使用如下命令，可查询到 RDMA 设备：

    确认能查询到 RoCE 网卡

        ~# rdma link
        link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
        link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: Ethernet

    确认主机上的 RDMA 子系统工作在 shared 模式下，否则，请切换到 shared 模式。

        ~# rdma system
        netns shared copy-on-fork on

        # 切换到 shared 模式
        ~# rdma system set netns shared

2. 确认 RDMA 网卡的信息，用于后续安装 device plugin

    本演示环境，输入如下命令，网卡 vendors 为 15b3，网卡 deviceIDs 为 1017

        ~# lspci -nn | grep Ethernet
        af:00.0 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        af:00.1 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

3. 安装 Spiderpool

        helm upgrade --install spiderpool spiderpool/spiderpool --namespace kube-system  --reuse-values \
           --set rdma.rdmaSharedDevicePlugin.install=true \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.resourcePrefix="spidernet.io" \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.resourceName="hca_shared_devices" \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.rdmaHcaMax=500 \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.vendors="15b3" \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.deviceIDs="1017"

    > - 如果您的集群未安装 Macvlan CNI, 可指定 Helm 参数 `--set plugins.installCNI=true` 安装 Macvlan 到每个节点。
    >
    > - 如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。
    
    完成 Spiderpool 安装后，可以手动编辑 configmap spiderpool-rdma-shared-device-plugin 来重新配置 RDMA shared device plugin。

    完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        spiderpool-rdma-shared-device-plugin-dr7w8     1/1     Running     0          1m
        spiderpool-rdma-shared-device-plugin-zj65g     1/1     Running     0          1m

4. 查看 node 的可用资源，其中包含了上报的 RDMA 设备资源：

        ~# kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
          [
            {
              "name": "10-20-1-10",
              "allocable": {
                "cpu": "40",
                "memory": "263518036Ki",
                "pods": "110",
                "spidernet.io/hca_shared_devices": "500",
                ...
              }
            },
            ...
          ]     

    > 如果上报的资源数为 0，可能的原因：
    >
    > (1) 请确认 configmap spiderpool-rdma-shared-device-plugin 中的 vendors 和 deviceID 与实际相符
    >
    > (2) 查看 rdma-shared-device-plugin 的日志，对于支持 RDMA 网卡报错如下日志，可尝试在主机上安装 apt-get install rdma-core 或 dnf install rdma-core
    >
    >   `error creating new device: "missing RDMA device spec for device 0000:04:00.0, RDMA device \"issm\" not found"`

5. 创建 macvlan 相关的 multus 配置，并创建配套的 ippool 资源

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-81
        spec:
          gateway: 172.81.0.1
          ips:
          - 172.81.0.100-172.81.0.120
          subnet: 172.81.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: macvlan-ens6f0np0
          namespace: kube-system
        spec:
          cniType: macvlan
          macvlan:
            master:
            - "ens6f0np0"
            ippools:
              ipv4: ["v4-81"]
        EOF

6. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用，用于测试

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/macvlan-ens6f0np0"
        RESOURCE="spidernet.io/hca_shared_devices"
        NAME=rdma-macvlan
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
              - image: docker.io/mellanox/rping-test
                imagePullPolicy: IfNotPresent
                name: mofed-test
                securityContext:
                  capabilities:
                    add: [ "IPC_LOCK" ]
                resources:
                  limits:
                    ${RESOURCE}: 1
                command:
                - sh
                - -c
                - |
                  ls -l /dev/infiniband /sys/class/net
                  sleep 1000000
        EOF

7. 在跨节点的 Pod 之间，确认 RDMA 收发数据正常。

    开启一个终端，进入一个 Pod 启动服务：

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        0/1: mlx5_0/1: state ACTIVE physical_state LINK_UP
        1/1: mlx5_1/1: state ACTIVE physical_state LINK_UP
        
        # 启动一个 RDMA 服务
        ~# ib_read_lat

    开启一个终端，进入另一个 Pod 访问服务：

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        0/1: mlx5_0/1: state ACTIVE physical_state LINK_UP
        1/1: mlx5_1/1: state ACTIVE physical_state LINK_UP
        
        # 访问对方 Pod 的服务
        ~# ib_read_lat 172.81.0.120
        ---------------------------------------------------------------------------------------
                            RDMA_Read Latency Test
         Dual-port       : OFF    Device         : mlx5_0
         Number of qps   : 1    Transport type : IB
         Connection type : RC   Using SRQ      : OFF
         TX depth        : 1
         Mtu             : 1024[B]
         Link type       : Ethernet
         GID index       : 12
         Outstand reads  : 16
         rdma_cm QPs   : OFF
         Data ex. method : Ethernet
        ---------------------------------------------------------------------------------------
         local address: LID 0000 QPN 0x0107 PSN 0x79dd10 OUT 0x10 RKey 0x1fddbc VAddr 0x000000023bd000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:119
         remote address: LID 0000 QPN 0x0107 PSN 0x40001a OUT 0x10 RKey 0x1fddbc VAddr 0x00000000bf9000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:120
        ---------------------------------------------------------------------------------------
         #bytes #iterations    t_min[usec]    t_max[usec]  t_typical[usec]    t_avg[usec]    t_stdev[usec]   99% percentile[usec]   99.9% percentile[usec]
        Conflicting CPU frequency values detected: 2200.000000 != 1040.353000. CPU Frequency is not max.
        Conflicting CPU frequency values detected: 2200.000000 != 1849.351000. CPU Frequency is not max.
         2       1000          6.88           16.81        7.04              7.06         0.31      7.38        16.81
        ---------------------------------------------------------------------------------------

## 基于 SR-IOV 隔离使用 RDMA 网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 SR-IOV CNI 给 Pod 分配隔离的 RDMA 设备：

1. 在宿主机上，确保主机拥有 RDMA 和 SR-IOV 功能的网卡。

    本示例环境中，宿主机上接入了 mellanox ConnectX 5 VPI 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。

    确认能查询到 RoCE 网卡

        ~# rdma link
        link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
        link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: Ethernet

    确认主机上的 RDMA 子系统工作在 exclusive 模式下，否则，请切换到 exclusive 模式。

        # 切换到 exclusive 模式，重启主机失效 
        ~# rdma system set netns exclusive
        # 持久化配置
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

        ~# rdma system
        netns exclusive copy-on-fork on

    （可选）SR-IOV 场景下，应用可使 NVIDIA 的 GPUDirect RMDA 功能，可参考 [官方文档](https://network.nvidia.com/products/GPUDirect-RDMA/) 安装内核模块。

2. 安装 Spiderpool

    - 务必设置 helm 选项 `--set sriov.install=true`
   
    - 如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。
    
    完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. 配置 SR-IOV operator

    给希望运行 SR-IOV CNI 的节点，按照如下命令打上如下 label。这样，sriov-network-operator 才会在指定的节点上安装组件:

        kubectl label node $NodeName node-role.kubernetes.io/worker=""

    查询 RDMA 网卡的设备信息。本演示环境，输入如下命令，网卡 vendors 为 15b3，网卡 deviceIDs 为 1017

        ~# lspci -nn | grep Ethernet
        af:00.0 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        af:00.1 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

    VF 的数量决定了一个主机上能同时为多少个 POD 提供 SR-IOV 网卡，不同厂商的网卡型号有不同的最大 VF 数量限制，例如本例使用的 Mellanox connectx5 能最多创建 127 个 VF。

    如下示例，写入正确的网卡的设备信息，使得 SR-IOV operator 能够在宿主机上创建出 VF，并上报资源。注意，该操作会配置网卡驱动配置，可能会引起相关主机重启。

        cat <<EOF | kubectl apply -f -
        apiVersion: sriovnetwork.openshift.io/v1
        kind: SriovNetworkNodePolicy
        metadata:
          name: roce-sriov
          namespace: kube-system
        spec:
          nodeSelector:
            kubernetes.io/os: "linux"
          resourceName: mellanoxrdma
          priority: 99
          numVfs: 12
          nicSelector:
              deviceID: "1017"
              rootDevices:
              - 0000:af:00.0
              vendor: "15b3"
          deviceType: netdevice
          isRdma: true
        EOF

    查看 node 的可用资源，其中包含了上报的 SR-IOV 设备资源

        ~# kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
        [
          {
            "name": "10-20-1-10",
            "allocable": {
              "cpu": "40",
              "pods": "110",
              "spidernet.io/mellanoxrdma": "12",
              ...
            }
          },
          ...
        ]

4. 创建 SR-IOV 相关的 multus 配置，并创建配套的 ippool 资源

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-81
        spec:
          gateway: 172.81.0.1
          ips:
          - 172.81.0.100-172.81.0.120
          subnet: 172.81.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: roce-sriov
          namespace: kube-system
        spec:
          cniType: sriov
          sriov:
            resourceName: spidernet.io/mellanoxrdma
            enableRdma: true
            ippools:
              ipv4: ["v4-81"]
        EOF

5. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用，用于测试

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/roce-sriov"
        RESOURCE="spidernet.io/mellanoxrdma"
        NAME=rdma-sriov
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
              - image: docker.io/mellanox/rping-test
                imagePullPolicy: IfNotPresent
                name: mofed-test
                securityContext:
                  capabilities:
                    add: [ "IPC_LOCK" ]
                resources:
                  limits:
                    ${RESOURCE}: 1
                command:
                - sh
                - -c
                - |
                  ls -l /dev/infiniband /sys/class/net
                  sleep 1000000
        EOF

6. 在跨节点的 Pod 之间，确认 RDMA 收发数据正常

    开启一个终端，进入一个 Pod 启动服务：

        # 只能看到分配给 Pod 的一个 RDMA 设备
        ~# rdma link
        7/1: mlx5_3/1: state ACTIVE physical_state LINK_UP netdev eth0
        
        # 启动一个 RDMA 服务
        ~# ib_read_lat

    开启一个终端，进入另一个 Pod 访问服务：

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        10/1: mlx5_5/1: state ACTIVE physical_state LINK_UP netdev eth0
        
        # 访问对方 Pod 的服务
        ~# ib_read_lat 172.81.0.118
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_4'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_2'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_0'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_3'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_1'.
        ---------------------------------------------------------------------------------------
                            RDMA_Read Latency Test
         Dual-port       : OFF    Device         : mlx5_5
         Number of qps   : 1    Transport type : IB
         Connection type : RC   Using SRQ      : OFF
         TX depth        : 1
         Mtu             : 1024[B]
         Link type       : Ethernet
         GID index       : 2
         Outstand reads  : 16
         rdma_cm QPs   : OFF
         Data ex. method : Ethernet
        ---------------------------------------------------------------------------------------
         local address: LID 0000 QPN 0x0b69 PSN 0xd476c2 OUT 0x10 RKey 0x006f00 VAddr 0x00000001f91000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:105
         remote address: LID 0000 QPN 0x0d69 PSN 0xbe5c89 OUT 0x10 RKey 0x004f00 VAddr 0x0000000160d000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:118
        ---------------------------------------------------------------------------------------
         #bytes #iterations    t_min[usec]    t_max[usec]  t_typical[usec]    t_avg[usec]    t_stdev[usec]   99% percentile[usec]   99.9% percentile[usec]
        Conflicting CPU frequency values detected: 2200.000000 != 1338.151000. CPU Frequency is not max.
        Conflicting CPU frequency values detected: 2200.000000 != 2881.668000. CPU Frequency is not max.
         2       1000          6.66           20.37        6.74              6.82         0.78      7.15        20.37
        ---------------------------------------------------------------------------------------
