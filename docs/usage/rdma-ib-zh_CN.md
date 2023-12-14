# RDMA with Infiniband

**简体中文** | [**English**](./rdma-ib.md)

## 介绍

Spiderpool 赋能了 [IB-SRIOV](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 和 [IPoIB](https://github.com/Mellanox/ipoib-cni) CNI， 这些 CNI 能让宿主机的 Infiniband 网卡暴露给 Pod 来使用。

## 功能

不同于基于 RoCE 网卡，Infiniband 网卡是基于 Infiniband 网络的专有设备，Spiderpool 提供了两种 CNI 选项：

1. 基于 [IB-SR-IOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 给 POD 提供 SR-IOV 网卡，并提供网络命名空间隔离的 RDMA 网卡。它适用于需要 RDMA 通信能力的 workload

2. 基于 [IPoIB CNI](https://github.com/Mellanox/ipoib-cni) 给 POD 提供 IPoIB 的网卡，它并不提供 RDMA 网卡通信能力，适用于需要 TCP/IP 通信的常规应用，因为它不需要提供 SRIOV 网卡，因此能让主机上运行更多 POD

并且，在 RDMA 通信场景下，对于基于 clusterIP 进行通信的应用，为了实现让 RDMA 流量通过 underlay 网卡转发，可在容器网络命名空间内基于 cgroup eBPF 实现的 clusterIP 的解析，具体可参考 [cgroup eBPF 解析 clusterIP](./underlay_cni_service-zh_CN.md)

### 基于 IB-SRIOV 提供 RDMA 网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 [IB-SRIOV](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 使得 Pod 接入 SRIOV 网卡，并提供网络命名空间隔离的 RDMA 设备：

1. 在宿主机上，确保主机拥有 RDMA 和 SR-IOV 功能的 Infiniband 网卡，且安装好驱动，确保 RDMA 功能工作正常。

    本示例环境中，宿主机上具备 RoCE 功能的 mellanox ConnectX 5 VPI 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。

    > 要隔离使用 RDMA 网卡，务必满足如下其中一个条件：
    >
    > (1) 内核版本要求 5.3.0 或更高版本，并在系统中加载 RDMA 模块。rdma-core 软件包提供了在系统启动时自动加载相关模块的功能。
    >
    > (2) Mellanox OFED 要求 4.7 或更高版本。此时不需要使用 5.3.0 或更新版本的内核。

    使用如下命令，查询主机上是否具备 Infiniband 网卡设备，并记录网卡的型号信息，用于后续创建 VF ：

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

    确认主机上的 RDMA 子系统工作在 exclusive 模式下，否则，请切换到 exclusive 模式。

        # 切换到 exclusive 模式，重启主机失效 
        ~# rdma system set netns exclusive
        # 持久化配置
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

        ~# rdma system
        netns exclusive copy-on-fork on

    确认 Infiniband 网卡具备 SR-IOV 功能，查看支持的最大 VF 数量：

        ~# cat /sys/class/net/ibs5f0/device/sriov_totalvfs
        127

    （可选）SR-IOV 场景下，应用可使 NVIDIA 的 GPUDirect RMDA 功能，可参考 [官方文档](https://network.nvidia.com/products/GPUDirect-RDMA/) 安装内核模块。

2. 安装好 Spiderpool，确认如下 helm 选项

    > - 务必开启 --set sriov.install=true 选项
    >
    > - 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。

    完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. 配置 SR-IOV operator

    如下配置，使得 SR-IOV operator 能够在宿主机上创建出 VF，并上报资源

        cat <<EOF | kubectl apply -f -
        apiVersion: sriovnetwork.openshift.io/v1
        kind: SriovNetworkNodePolicy
        metadata:
          name: ibsriov
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

4. 创建 IB-SRIOV 的 CNI 配置，并创建配套的 ippool 资源

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-91
        spec:
          gateway: 172.91.0.1
          ips:
            - 172.91.0.100-172.91.0.120
          subnet: 172.91.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ib-sriov
          namespace: kube-system
        spec:
          cniType: ib-sriov
          ibsriov:
            resourceName: spidernet.io/mellanoxibsriov
            ippools:
              ipv4: ["v4-91"]
        EOF

5. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/ib-sriov"
        RESOURCE="spidernet.io/mellanoxibsriov"
        NAME=ib-sriov
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
        link mlx5_4/1 subnet_prefix fe80:0000:0000:0000 lid 8 sm_lid 1 lmc 0 state ACTIVE physical_state LINK_UP
        
        # 启动一个 RDMA 服务
        ~# ib_read_lat

    开启一个终端，进入另一个 Pod 访问服务：

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        link mlx5_8/1 subnet_prefix fe80:0000:0000:0000 lid 7 sm_lid 1 lmc 0 state ACTIVE physical_state LINK_UP
        
        # 访问对方 Pod 的 RDMA 服务
        ~# ib_read_lat 172.91.0.115
        ---------------------------------------------------------------------------------------
                            RDMA_Read Latency Test
        Dual-port       : OFF		Device         : mlx5_8
        Number of qps   : 1		Transport type : IB
        Connection type : RC		Using SRQ      : OFF
        PCIe relax order: ON
        ibv_wr* API     : ON
        TX depth        : 1
        Mtu             : 4096[B]
        Link type       : IB
        Outstand reads  : 16
        rdma_cm QPs	 : OFF
        Data ex. method : Ethernet
        ---------------------------------------------------------------------------------------
        local address: LID 0x07 QPN 0x012e PSN 0x7eb74 OUT 0x10 RKey 0x030509 VAddr 0x005560e826f000
        remote address: LID 0x08 QPN 0x00ee PSN 0x7eb74 OUT 0x10 RKey 0x020509 VAddr 0x005560f99dc000
        ---------------------------------------------------------------------------------------
        #bytes #iterations    t_min[usec]    t_max[usec]  t_typical[usec]    t_avg[usec]    t_stdev[usec]   99% percentile[usec]   99.9% percentile[usec]
        Conflicting CPU frequency values detected: 1000.085000 != 2200.000000. CPU Frequency is not max.
        Conflicting CPU frequency values detected: 1000.383000 != 2200.000000. CPU Frequency is not max.
        2       1000          1.84           12.20        1.90     	       1.97        	0.47   		2.24    		12.20
        ---------------------------------------------------------------------------------------

### 基于 IPoIB 的常规网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 [IPoIB](https://github.com/Mellanox/ipoib-cni) 使得 Pod 接入常规的 TCP/IP 网卡，使用应用能够在 Infiniband 网络中进行 TCP/IP 通信：

1. 在宿主机上，确保主机拥有 Infiniband 网卡，且安装好驱动。

    本示例环境中，宿主机上具备 RoCE 功能的 mellanox ConnectX 5 VPI 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。

    确认主机上查看 Infiniband 网卡的 IPoIB 接口
        ~# ip a show ibs5f0
        9: ibs5f0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 2044 qdisc mq state UP group default qlen 256
        link/infiniband 00:00:10:49:fe:80:00:00:00:00:00:00:e8:eb:d3:03:00:93:ae:10 brd 00:ff:ff:ff:ff:12:40:1b:ff:ff:00:00:00:00:00:00:ff:ff:ff:ff
        altname ibp134s0f0
        inet 172.91.0.10/16 brd 172.91.255.255 scope global ibs5f0
        valid_lft forever preferred_lft forever
        inet6 fd00:91::172:91:0:10/64 scope global
        valid_lft forever preferred_lft forever
        inet6 fe80::eaeb:d303:93:ae10/64 scope link
        valid_lft forever preferred_lft forever

2. 安装好 Spiderpool，确认如下安装选项

    > - 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。

   完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. 创建 ipoib 的 CNI 配置，并创建配套的 ippool 资源

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-91
        spec:
          gateway: 172.91.0.1
          ips:
            - 172.91.0.100-172.91.0.120
          subnet: 172.91.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ipoib
          namespace: kube-system
        spec:
          cniType: ib-sriov
          ipoib:
            master: "ibs5f0"
            ippools:
              ipv4: ["v4-91"]
        EOF

4. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/ipoib"
        NAME=ipoib
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
                command:
                - sh
                - -c
                - |
                  ls -l /dev/infiniband /sys/class/net
                  sleep 1000000
        EOF

5. 在跨节点的 Pod 之间，确认应用之间能正常 TCP/IP 通信

        ~# kubectl get pod -o wide
        NAME                         READY   STATUS             RESTARTS          AGE    IP             NODE         NOMINATED NODE   READINESS GATES
        ipoib-psf4q                  1/1     Running            0                 34s    172.91.0.112   10-20-1-20   <none>           <none>
        ipoib-t9hm7                  1/1     Running            0                 34s    172.91.0.116   10-20-1-10   <none>           <none>

    从一个 POD 中成功访问另一个 POD

        ~# kubectl exec -it ipoib-psf4q bash
        kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
        root@ipoib-psf4q:/# ping 172.91.0.116
        PING 172.91.0.116 (172.91.0.116) 56(84) bytes of data.
        64 bytes from 172.91.0.116: icmp_seq=1 ttl=64 time=1.10 ms
        64 bytes from 172.91.0.116: icmp_seq=2 ttl=64 time=0.235 ms
