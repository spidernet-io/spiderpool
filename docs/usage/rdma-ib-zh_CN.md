# RDMA with RoCE

**简体中文** | [**English**](./rdma-roce.md)

## 介绍

Spiderpool 赋能了 IB-SR-IOV 和 IPoIB CNI， 这些 CNI 能让宿主机的 RDMA 网卡暴露给 Pod 来使用，本章节将介绍在 Spiderpool 下如何使用基于 Infiniband 的 RDMA 网卡。

## 功能

RDMA 设备的网络命名空间具备 shared 和 exclusive 两种模式，容器因此可以实现共享 RDMA 网卡，或者独享 RDMA 网卡。在 kubernetes 下，可基于 macvlan 或 ipvlan CNI 来使用 shared 模式的 RoCE
RDMA 网卡，也可以基于 SR-IOV CNI 来使用 exclusive 模式的网卡。

在 shared 模式下，Spiderpool 使用了 macvlan 或 ipvlan CNI 来暴露宿主机上的 RoCE 网卡给 Pod 使用，使用 [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin) 来完成 RDMA 网卡资源的暴露和 Pod 调度。

在 exclusive 模式下，Spiderpool 使用了 [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-network-operator) 来暴露宿主机上的 RDMA 网卡给 Pod 使用，暴露 RDMA 资源。使用 [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni) 来完成 RDMA 设备隔离。

### 基于 SR-IOV 隔离使用 RDMA Infiniband 网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 [ib-sriov](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) CNI 使得 Pod 隔离使用 RDMA 设备：

1. 在宿主机上，确保主机拥有 RDMA 和 SR-IOV 功能的 Infiniband 网卡，且安装好驱动，确保 RDMA 功能工作正常。

    本示例环境中，宿主机上具备 RoCE 功能的 mellanox ConnectX 5 VPI 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。

    > 要隔离使用 RDMA 网卡，务必满足如下其中一个条件：
    >
    > (1) 内核版本要求 5.3.0 或更高版本，并在系统中加载 RDMA 模块。rdma-core 软件包提供了在系统启动时自动加载相关模块的功能。
    >
    > (2) Mellanox OFED 要求 4.7 或更高版本。此时不需要使用 5.3.0 或更新版本的内核。

    使用如下命令，可查询到 Infiniband 网卡设备：

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

    确认主机上的 RDMA 子系统工作在 exclusive 模式下，否则，请切换到 exclusive 模式。

        # 切换到 exclusive 模式，重启主机失效 
        ~# rdma system set netns exclusive
        # 持久化配置
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

        ~# rdma system
        netns exclusive copy-on-fork on

    确认网卡具备 SR-IOV 功能，查看支持的最大 VF 数量：

        ~# cat /sys/class/net/ibs5f0/device/sriov_totalvfs
        127

    （可选）SR-IOV 场景下，应用可使 NVIDIA 的 GPUDirect RMDA 功能，可参考 [官方文档](https://network.nvidia.com/products/GPUDirect-RDMA/) 安装内核模块。

2. 确认 RDMA 网卡的信息，用于后续 device plugin 发现设备资源。

    本演示环境，输入如下，网卡 vendors 为 15b3，网卡 deviceIDs 为 1017

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

3. 安装 Spiderpool

        helm repo add spiderpool https://spidernet-io.github.io/spiderpool
        helm repo update spiderpool
        helm install spiderpool spiderpool/spiderpool -n kube-system \
           --set sriov.install=true  \
           --set multus.multusCNI.defaultCniCRName="ibsriov-rdma"

    > - 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。
    >
    > - 通过 `multus.multusCNI.defaultCniCRName` 指定 multus 默认使用的 CNI 的 NetworkAttachmentDefinition 实例名。如果 `multus.multusCNI.defaultCniCRName` 选项不为空，则安装后会自动生成一个数据为空的 NetworkAttachmentDefinition 对应实例。如果 `multus.multusCNI.defaultCniCRName` 选项为空，会尝试通过 /etc/cni/net.d 目录下的第一个 CNI 配置来创建对应的 NetworkAttachmentDefinition 实例，否则会自动生成一个名为 `default` 的 NetworkAttachmentDefinition 实例，以完成 multus 的安装。

    完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

4. 配置 SR-IOV operator

    如下配置，使得 SR-IOV operator 能够在宿主机上创建出 VF，并上报资源

        cat <<EOF | kubectl apply -f -
        apiVersion: sriovnetwork.openshift.io/v1
        kind: SriovNetworkNodePolicy
        metadata:
          name: policyibsriov
          namespace: kube-system
        spec:
          nodeSelector:
            kubernetes.io/os: "linux"
          resourceName: mellanoxibsriov
          priority: 99
          numVfs: 12
          nicSelector:
              deviceID: "1017"
              rootDevices:
              - 0000:86:00.0
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
              "spidernet.io/mellanoxibsriov": "12",
              ...
            }
          },
          ...
        ]

5. 创建 SR-IOV 相关的 multus 配置，并创建配套的 ippool 资源

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
          name: ib-sriov
          namespace: kube-system
        spec:
          cniType: ib-sriov
          sriov:
            resourceName: spidernet.io/mellanoxibsriov
            ippools:
              ipv4: ["v4-81"]
        EOF

6. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/ib-sriov"
        RESOURCE="spidernet.io/mellanoxibsriov"
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

7. 在跨节点的 Pod 之间，确认 RDMA 收发数据正常

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
