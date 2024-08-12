# IPoIB For Infiniband

**简体中文** | [**English**](./ipoib.md)

## 介绍

本节介绍基于主机上的 Infiniband 网卡，如何给 POD 分配 IPoIB 网卡，使得传统的 TCP/IP 应用能够在 Infiniband 网络中工作。

Spiderpool 基于 [IPoIB CNI](https://github.com/Mellanox/ipoib-cni) 给 POD 提供 IPoIB 的网卡，它并不提供 RDMA 网卡通信能力，适用于需要 TCP/IP 通信的常规应用。

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
