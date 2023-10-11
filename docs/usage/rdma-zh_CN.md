# RDMA

**简体中文** | [**English**](./rdma.md)

## 介绍

Spiderpool 赋能了 macvlan ipvlan 和 SRIOV CNI， 这些 CNI 能让宿主机的 RDMA 网卡暴露给 POD 来使用，本章节将介绍在 Spiderpool 下如何 RDMA 网卡。

## 功能

RDMA 设备的网络命名空间具备 shared 和 exclusive 两种模式，容器因此可以实现共享 RDMA 网卡，或者独享 RDMA 网卡。在 kubernetes 下，可基于 macvlan 或 ipvlan CNI 来使用 shared 模式的
RDMA 网卡，也可以基于 SRIOV CNI 来使用 exclusive 模式的网卡。

在 shared 模式下，Spiderpool 使用了 macvlan 或 ipvlan CNI 来暴露宿主机上的 RoCE 网卡给 PDO 使用，使用 [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin) 来完成 RDMA 网卡资源的暴露和 POD 调度。

在 exclusive 模式下，Spiderpool 使用了 [SRIOV CNI](https://github.com/k8snetworkplumbingwg/sriov-network-operator) 来暴露宿主机上的 RDMA 网卡给 PDO 使用，暴露 RDMA 资源。使用 [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni) 来完成 RDMA 设备隔离。

### 基于 macvlan 或 ipvlan 共享使用具备 RoCE 功能的网卡

以下步骤，在具备 2 个节点的集群上，演示如何基于 macvlan CNI 使得 POD 共享使用 RDMA 设备

1. 在宿主机上，确保主机拥有 RDMA 网卡，且安装好驱动，确保 RDMA 功能工作正常。

   本示例环境中，宿主机上具备 RoCE 功能的 mellanox ConnectX 5 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动，。使用如下命令，可查询到 rdma 设备

        ~# rdma link show
        link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
        link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1

   确认主机上确认 rdma 子系统工作在 shared 模式下，否则，请切换到 模式，

        ~# rdma system
        netns shared copy-on-fork on

        # 切换到 shared 模式
        ~# rdma system set netns shared 

2. 确认 RDMA 网卡的信息，用于后续 device plugin 发现设备资源。

   本演示环境，输入如下，网卡 vendors 为 15b3，网卡 deviceIDs 为 1017

        ~# lspci -nn | grep Ethernet
        af:00.0 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        af:00.1 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

3. 可参考 [安装](./install/underlay/get-started-macvlan-zh_CN.md) 安装 Spiderpool 并配置 sriov-network-operator。其中，按照命令务必加上如下 helm 选项来安装 [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)

        helm install spiderpool spiderpool/spiderpool -n kube-system \
          --set multus.multusCNI.defaultCniCRName="macvlan-conf" \
          --set rdma.rdmaSharedDevicePlugin.install=true \
          --set rdma.rdmaSharedDevicePlugin.deviceConfig.resourcePrefix="spidernet.io" \
          --set rdma.rdmaSharedDevicePlugin.deviceConfig.resourceName="hca_shared_devices" \
          --set rdma.rdmaSharedDevicePlugin.deviceConfig.rdmaHcaMax=500 \
          --set rdma.rdmaSharedDevicePlugin.deviceConfig.vendors="15b3" \
          --set rdma.rdmaSharedDevicePlugin.deviceConfig.deviceIDs="1017"

   > 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。
   >
   > 注：完成 spiderpool 安装后，可以手动编辑 configmap spiderpool-rdma-shared-device-plugin 来重新配置 RDMA shared device plugin

   完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        spiderpool-multus-ckjrl                        1/1     Running     0          1m
        spiderpool-multus-mjl7z                        1/1     Running     0          1m
        spiderpool-rdma-shared-device-plugin-dr7w8     1/1     Running     0          1m
        spiderpool-rdma-shared-device-plugin-zj65g     1/1     Running     0          1m

5. 查看 node 的可用资源，其中包含了上报的 rdma 设备资源

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

6. 基于 RDMA 网卡作为 master 节点，创建 macvlan 相关的 multus 配置，并创建配套的 ippool 资源

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

7. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用

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

8. 在跨加点的 POD 之间，确认 RDMA 收发数据正常

   开启一个终端，进入一个 POD 启动服务

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        0/1: mlx5_0/1: state ACTIVE physical_state LINK_UP
        1/1: mlx5_1/1: state ACTIVE physical_state LINK_UP
        
        # 启动一个 RDMA 服务
        ~# ib_read_lat

   开启一个终端，进入另一个 POD 访问服务

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        0/1: mlx5_0/1: state ACTIVE physical_state LINK_UP
        1/1: mlx5_1/1: state ACTIVE physical_state LINK_UP
        
        # 访问对方 POD 的服务
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

### 基于 SRIOV 隔离使用 RDMA 网卡

以下步骤，在具备 2 个节点的集群上，演示如何基于 SRIOV CNI 使得 POD 隔离使用 RDMA 设备

1. 在宿主机上，确保主机拥有 RDMA 和 SRIOV 功能的网卡，且安装好驱动，确保 RDMA 功能工作正常。

   本示例环境中，宿主机上具备 RoCE 功能的 mellanox ConnectX 5 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动，。

   > 注意：要隔离使用 RDMA 网卡，务必满足如下其中一个条件
   > (1) Kernel based on 5.3.0 or newer, RDMA modules loaded in the system. rdma-core package provides means to automatically load relevant modules on system start
   > (2) Mellanox OFED version 4.7 or newer is required. In this case it is not required to use a Kernel based on 5.3.0 or newer.

   使用如下命令，可查询到 rdma 设备

        ~# rdma link show
        link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
        link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1

   确认主机上确认 rdma 子系统工作在 shared 模式下，否则，请切换到 模式，

        # 切换到 exclusive 模式，重启重启失效 
        ~# rdma system set netns exclusive

        # 持久化配置
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

        ~# rdma system
        netns exclusive copy-on-fork on

   确认网卡具备 SRIOV 功能，查看支持的最大 VF 数量
   ~# cat /sys/class/net/ens6f0np0/device/sriov_totalvfs
   127

   （可选）SRIOV 场景下，应用可使 NVIDIA 的 GPUDirect RMDA 功能，可参考 [官方文档](https://network.nvidia.com/products/GPUDirect-RDMA/) 安装内核模块。

2. 确认 RDMA 网卡的信息，用于后续 device plugin 发现设备资源。

   本演示环境，输入如下，网卡 vendors 为 15b3，网卡 deviceIDs 为 1017

        ~# lspci -nn | grep Ethernet
        af:00.0 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        af:00.1 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

3. 可参考 [安装](./install/underlay/get-started-sriov-zh_CN.md) 安装 Spiderpool，其中，务必加上如下 helm 选项来安装 [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni)

        helm install spiderpool spiderpool/spiderpool -n kube-system \
          --set sriov.install=true  \
          --set rdma.rdmaCni.install=true

   > 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。
   >
   > 注：完成 spiderpool 安装后，可以手动编辑 configmap spiderpool-rdma-shared-device-plugin 来重新配置 RDMA shared device plugin

   完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        spiderpool-multus-ckjrl                        1/1     Running     0          1m
        spiderpool-multus-mjl7z                        1/1     Running     0          1m

5. 配置 SRIOV operator

   如下配置，使得 SRIOV operator 能够在宿主机上创建出 VF，并上报资源
   cat <<EOF | kubectl apply -f -
   apiVersion: sriovnetwork.openshift.io/v1
   kind: SriovNetworkNodePolicy
   metadata:
   name: policyrdma
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

   查看 node 的可用资源，其中包含了上报的 SRIOV 设备资源

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

6. 创建 SRIOV 相关的 multus 配置，并创建配套的 ippool 资源

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
          name: sriov-rdma
          namespace: kube-system
        spec:
          cniType: sriov
          sriov:
            resourceName: spidernet.io/mellanoxrdma
            enableRdma: true
            ippools:
              ipv4: ["v4-81"]
        EOF

7. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/sriov-rdma"
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

8. 在跨加点的 POD 之间，确认 RDMA 收发数据正常

   开启一个终端，进入一个 POD 启动服务

        # 只能看到分配给 POD 的一个 RDMA 设备
        ~# rdma link
        7/1: mlx5_3/1: state ACTIVE physical_state LINK_UP netdev eth0
        
        # 启动一个 RDMA 服务
        ~# ib_read_lat

   开启一个终端，进入另一个 POD 访问服务

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        10/1: mlx5_5/1: state ACTIVE physical_state LINK_UP netdev eth0
        
        # 访问对方 POD 的服务
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
