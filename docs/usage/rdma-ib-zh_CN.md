# RDMA with Infiniband

**简体中文** | [**English**](./rdma-ib.md)

## 介绍

本节介绍基于主机上的 Infiniband 网卡，如何给 POD 分配网卡。

## 功能

不同于 RoCE 网卡，Infiniband 网卡是基于 Infiniband 网络的专有设备，Spiderpool 提供了两种 CNI 选项：

1. 基于 [IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 给 POD 提供 SR-IOV 网卡。它适用于需要 RDMA 通信能力的 workload

    它提供两种 RDMA 模式：

    - 共享模式，POD 将具有 RDMA 功能的 SR-IOV 网络接口，但运行在同一节点中的所有 Pod 都可以看到所有 RDMA 设备。POD可能会混淆应使用 RDMA 设备

    - 独占模式，POD 将有一个具有 RDMA 功能的 SR-IOV 网络接口，且 POD 只能查看自己的 RDMA 设备，并不会产生混淆。
   
        对于隔离 RDMA 网卡，必须至少满足以下条件之一：

        （1） 基于 5.3.0 或更新版本的 Linux 内核，系统中加载的RDMA模块，rdma 核心包提供了在系统启动时自动加载相关模块的方法

        （2） 需要 Mellanox OFED 4.7 版或更新版本。在这种情况下，不需要使用基于 5.3.0 或更新版本的内核。

2. 基于 [IPoIB CNI](https://github.com/Mellanox/ipoib-cni) 给 POD 提供 IPoIB 的网卡，它并不提供 RDMA 网卡通信能力，适用于需要 TCP/IP 通信的常规应用，因为它不需要提供 SRIOV 资源，因此能让主机上运行更多 POD

## 基于 IB-SRIOV 提供 RDMA 网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 [IB-SRIOV](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 使得 Pod 接入 SRIOV 网卡，并提供网络命名空间隔离的 RDMA 设备：

1. 在宿主机上，确保主机拥有 RDMA 和 SR-IOV 功能的 Infiniband 网卡。

    本示例环境中，宿主机上接入了 mellanox ConnectX 5 VPI 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。

    对于 mellanox 的 VPI 系列网卡，可参考官方的 [切换 Infiniband 模式](https://support.mellanox.com/s/article/MLNX2-117-1997kn)，确保网卡工作在 Infiniband 模式下。

    使用如下命令，查询主机上是否具备 Infiniband 网卡设备 ：

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

        ~# rdma link
        link mlx5_0/1 subnet_prefix fe80:0000:0000:0000 lid 2 sm_lid 2 lmc 0 state ACTIVE physical_state LINK_UP

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: InfiniBand

    确认主机上的 RDMA 子系统工作在 exclusive 模式下，否则，请切换到 exclusive 模式。

        # 切换到 exclusive 模式，重启主机失效 
        ~# rdma system set netns exclusive
        # 持久化配置
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf
        ~# reboot

        ~# rdma system
        netns exclusive copy-on-fork on

    > 如果希望工作在 shared 模式下，可输入命令 `rm /etc/modprobe.d/ib_core.conf && reboot`

    （可选）SR-IOV 场景下，应用可使 NVIDIA 的 GPUDirect RMDA 功能，可参考 [官方文档](https://network.nvidia.com/products/GPUDirect-RDMA/) 安装内核模块。

2. 安装好 Spiderpool，确认如下 helm 选项

        helm upgrade --install spiderpool spiderpool/spiderpool --namespace kube-system  --reuse-values --set sriov.install=true
    
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

    使用如下命令，查询主机上 Infiniband 网卡设备的信息

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

    VF 的数量决定了一个主机上能同时为多少个 POD 提供 SR-IOV 网卡，不同厂商的网卡型号有不同的最大 VF 数量限制，例如本例使用的 Mellanox connectx5 能最多创建 127 个 VF。

    如下示例，写入正确的网卡的设备信息，使得 SR-IOV operator 能够在宿主机上创建出 VF，并上报资源。注意，该操作会配置网卡驱动配置，可能会引起相关主机重启。

        cat <<EOF | kubectl apply -f -
        apiVersion: sriovnetwork.openshift.io/v1
        kind: SriovNetworkNodePolicy
        metadata:
          name: ib-sriov
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

5. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用，进行测试

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

    开启一个终端，进入一个 Pod 启动服务

        # 只能看到分配给 Pod 的一个 RDMA 设备
        ~# rdma link
        link mlx5_4/1 subnet_prefix fe80:0000:0000:0000 lid 8 sm_lid 1 lmc 0 state ACTIVE physical_state LINK_UP

        # 启动一个 RDMA 服务
        ~# ib_read_lat

    开启一个终端，进入另一个 Pod 访问服务：

        # 能看到宿主机上的所有 RDMA 网卡
        ~# rdma link
        link mlx5_8/1 subnet_prefix fe80:0000:0000:0000 lid 7 sm_lid 1 lmc 0 state ACTIVE physical_state LINK_UP
        
        # 成功访问对方 Pod 的 RDMA 服务
        ~# ib_read_lat 172.91.0.115

7.【可选】对于具备 UFM 管理平台的环境，可结合 ib-kubernetes 与 UFM 完成 Infiniband 子网管理功能，参考章节 [UFM 和 IB-Kubernetes](#使用-ib-kubernetes-和-ufm-实现-infiniband-网络管理)。对于不具备 UFM 管理平台的环境，可忽略本步骤。

## 基于 IPoIB 的常规网卡

以下步骤演示在具备 2 个节点的集群上，如何基于 [IPoIB](https://github.com/Mellanox/ipoib-cni) 使得 Pod 接入常规的 TCP/IP 网卡，使得应用能够在 Infiniband 网络中进行 TCP/IP 通信，但是应用不能进行 RDMA 通信

1. 在宿主机上，确保主机拥有 Infiniband 网卡，且安装好驱动。

    本示例环境中，宿主机上接入了 mellanox ConnectX 5 VPI 网卡，可按照 [NVIDIA 官方指导](https://developer.nvidia.com/networking/ethernet-software) 安装最新的 OFED 驱动。

    对于 mellanox 的 VPI 系列网卡，可参考官方的 [切换 Infiniband 模式](https://support.mellanox.com/s/article/MLNX2-117-1997kn)，确保网卡工作在 Infiniband 模式下。

    使用如下命令，查询主机上是否具备 Infiniband 网卡设备 ：

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

        ~# rdma link
        link mlx5_0/1 subnet_prefix fe80:0000:0000:0000 lid 2 sm_lid 2 lmc 0 state ACTIVE physical_state LINK_UP

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: InfiniBand

    查看 Infiniband 网卡的 IPoIB 接口

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

2. 安装好 Spiderpool

    如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。

    完成后，安装的组件如下

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. 创建 ipoib 的 CNI 配置，并创建配套的 ippool 资源。其中 SpiderMultusConfig 的 spec.ipoib.master 指向主机上的 Infiniband 网卡

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
          cniType: ipoib
          ipoib:
            master: "ibs5f0"
            ippools:
              ipv4: ["v4-91"]
        EOF

4. 使用上一步骤的配置，来创建一组跨节点的 DaemonSet 应用进行测试

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

## 使用 IB-Kubernetes 和 UFM 实现 Infiniband 网络管理

[ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) 是 Mellanox 开源的一款 Kubernetes 的 Infiniband 插件，它与 [ib-sriov-cni](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 和 [Multus-cni](https://github.com/k8snetworkplumbingwg/multus-cni) 协同工作： 完成 Pkey 和 GUID 的设置，并通告给 UFM 类的插件， 再由 UFM 完成 Infiniband 网络下的子网管理。

[UFM](https://docs.nvidia.com/networking/display/ufmenterpriseqsgv6160/ufm+installation+steps) 是 Nvidia 用于管理 Infiniband 网络的软件，它可管理 Infiniband 的交换机以及主机。它具有以下的能力:

- 发现并管理 Infiniband 网络的主机、交换机、线缆、网关等网络设备，进行设备发现、拓扑展示、软件升级、日志采集等基础管理。
- 实施包括 pkey 在内的网络配置
- 网络设备状态监控，网络实时流量遥测，网络自检，实施告警
- opensm 管理网络设备

> 注： UFM 软件以 Server 形式运行在具有 Infiniband 网卡的主机上，需要购买 License 才能使用。

下面将介绍 UFM 和 ib-kubernetes 如何协同工作以及一些使用实例。

### 工作流程

![ib-kubernetes-ufm](../images/ib-kubernetes.png)

从上图可以看出：ib-kubernetes 会在 Pod 创建时，读取它的 multus 配置或 annotations，从而读取 pkey 或 guid(如有配置，没有将自动生成)，通过调用 UFM 插件的 API，将 pkey 和 guid 信息传递给 UFM 插件。然后 UFM 将根据二者信息完成对该 Pod 的子网管理功能。

### 如何使用

您需要提前在环境中安装好 UFM 插件，下面我们安装 ib-kubernetes:

1. 准备好插件配置，以下配置用于帮助登录 UFM 管理平台。

        apiVersion: v1
        kind: Secret
        metadata:
          name: ib-kubernetes-ufm-secret
          namespace: kube-system
        stringData:
          UFM_USERNAME: "admin"  # UFM 用户名
          UFM_PASSWORD: "123456" # UFM 密码
          UFM_ADDRESS: ""        # UFM 管理地址 
          UFM_HTTP_SCHEMA: ""    # http/https. Default: https
          UFM_PORT: ""           # UFM REST API port. Defaults: 443(https), 80(http)
        string:
          UFM_CERTIFICATE: ""    # UFM Certificate in base64 format. (if not provided client will not

2. 登录 UFM 需要以证书方式，需要先在 UFM 主机生成证书文件：

        $ openssl req -x509 -newkey rsa:4096 -keyout ufm.key -out ufm.crt -days 365 -subj '/CN=<UFM hostname>'

        将证书文件复制到 UFM 证书位置：

        $ cp ufm.key /etc/pki/tls/private/ufmlocalhost.key
        $ cp ufm.crt /etc/pki/tls/certs/ufmlocalhost.crt

        重启 UFM：

        $ docker restart ufm

        如果以裸金属部署：

        $ systemctl restart ufmd

3. 创建 UFM 的证书密钥文件：

        $ kubectl create secret generic ib-kubernetes-ufm-secret --namespace="kube-system" --from-literal=UFM_USER="admin" --from-literal=UFM_PASSWORD="12345" --from-literal=UFM_ADDRESS="127.0.0.1" --from-file=UFM_CERTIFICATE=ufmlocalhost.crt --dry-run -o yaml > ib-kubernetes-ufm-secret.yaml
        $ kubectl create -f ./ib-kubernetes-ufm-secret.yaml 

4. 安装 ib-kubernetes:

        $ git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
        $ $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
        $ kubectl create -f deployment/ib-kubernetes-ufm-secret.yaml
        $ kubectl create -f deployment/ib-kubernetes.yaml 

5. 创建 ib-sriov-cni 的 SpiderMultusConfig 时，配置 pkey 信息:

        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ib-sriov
          namespace: kube-system
        spec:
          cniType: ib-sriov
          ibsriov:
            resourceName: spidernet.io/mellanoxibsriov
            pkey: 1000
            ippools:
              ipv4: ["v4-91"]

### 结论

ib-kubernetes 与 UFM 软件集成，帮助 UFM 完成 Kubernetes 下 Infiniband 网络的子网管理。
