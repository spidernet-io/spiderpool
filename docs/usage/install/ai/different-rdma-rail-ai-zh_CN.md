# Different RDMA Zone AI Cluster With SR-IOV 

**简体中文** | [**English**](./large-scale-rdma-zone.md)

## 介绍

[AI Cluster With SR-IOV](./get-started-sriov-zh_CN.md)介绍了如何使用 Spiderpool 和 SR-IOV 组件来配置大规模 AI 集群的网络。 该文档适用于 AI 集群中所有节点的相同轨道网卡使用相同子网的场景。此种场景下，整个 AI 集群需要多个子网，比如如果节点拥有 8 个轨道的网卡，那么需要 8 个独立的子网。 在一些大规模 AI 集群中由于 IP 地址资源的限制，并不能提供这么多的子网。只能将有限的子网拆分给不同节点的不同轨道网卡使用，所以在此场景下，不同节点的相同轨道网卡往往被分配到不同的子网中。

假设我们拥有一个 16 位掩码的地址 IP 网段，每个节点每个轨道的网卡可有 32 个地址，则掩码为 27 位。在每个节点拥有 8 个轨道网卡，我们最多可支持 256 个节点（每个节点 8 个轨道网卡，每个轨道网卡 32 个地址）。

比如节点 node1 的 1 号轨道网卡可能使用 172.16.1.0/27 子网，而节点 node2 的 1 号轨道网卡可能使用 172.16.1.32/27 子网

此种模式具有以下特点：

- **配置复杂度高**：需要为每个节点的每个轨道网卡创建独立的 IP 池资源，需要保证 Pod 调度到节点上时，能够正确分配到该节点匹配的 IP 池。
- **RDMA 流量路径**：第一个 RDMA 网卡用于承接 RDMA 控制面通信，而不是默认的管理网卡，其他 RDMA 网卡用于承载 AI 计算的 RDMA 流量。

Spiderpool 支持 **基于主机 RDMA 轨道子网自动为 Pod 分配匹配的 IP 池** 功能，实现这种 AI 网络方案能够完美运行。

## 方案架构

![Large Scale RDMA Zone](../../../images/ai-different-zone.png)

如图 1 所示，集群的网络规划如下：

1. 每个节点拥有 8 个 RDMA 轨道网卡，每个轨道网卡可分配 32 个 IP 地址，掩码为 27 位， 每个节点的 8 个轨道网卡使用不同的子网
2. 每个节点的 nic0 网卡运行 calico CNI，来承载 kubernetes 流量。AI workload 将会被分配一个 calico 的缺省网卡，进行控制面通信。
3. 节点上使用具备 RDMA 功能的 Mellanox ConnectX5 网卡来承载 AI 计算的 RDMA 流量，网卡接入到 rail optimized 网络中。AI workload 将会被额外分配所有 RDMA 网卡的 SR-IOV 虚拟化接口，确保 GPU 的高速网络通信。注意：第一个 RDMA 网卡用于承接 RDMA 控制面通信，其他网卡用于承载 AI 计算的 RDMA 流量。

## 安装要求

- 参考 [Spiderpool安装要求](./../system-requirements-zh_CN.md)

- 主机上准备好 Helm 二进制

- 安装好 Kubernetes 集群，kubelet 工作在图 1 中的主机 eth0 网卡上

- 在 Infiniband 网络场景下，确保 OpenSM 子网管理器工作正常

- 安装 Calico 作为集群的缺省 CNI，使用主机的 eth0 网卡作为 calico 的流量转发网卡。
    如果未安装，可参考 [官方文档](https://docs.tigera.io/calico/latest/getting-started/kubernetes/) 或参考以下命令安装:

    ```shell
    $ kubectl apply -f https://github.com/projectcalico/calico/blob/master/manifests/calico.yaml
    $ kubectl wait --for=condition=ready -l k8s-app=calico-node  pod -n kube-system 
    # set calico to work on host eth0 
    $ kubectl set env daemonset -n kube-system calico-node IP_AUTODETECTION_METHOD=kubernetes-internal-ip
    # set calico to work on host eth0 
    $ kubectl set env daemonset -n kube-system calico-node IP6_AUTODETECTION_METHOD=kubernetes-internal-ip  
    ```

## 主机准备

1. 安装 RDMA 网卡驱动，然后重启主机（这样才能看到网卡）

    对于 Mellanox 网卡，可下载 [NVIDIA OFED 官方驱动](https://network.nvidia.com/products/infiniband-drivers/linux/mlnx_ofed/) 进行主机安装，执行如下安装命令

    ```shell
    mount /root/MLNX_OFED_LINUX-24.01-0.3.3.1-ubuntu22.04-x86_64.iso   /mnt
    /mnt/mlnxofedinstall --all
    ```

    对于 Mellanox 网卡，也可基于容器化安装，实现对集群主机上所有 Mellanox 网卡批量安装驱动，运行如下命令，注意的是，该运行过程中需要访问因特网获取一些安装包。当所有的 ofed pod 进入 ready 状态，表示主机上已经完成了 OFED driver 安装

    ```shell
    $ helm repo add spiderchart https://spidernet-io.github.io/charts
    $ helm repo update
    $ helm search repo ofed

    # pelase replace the following values with your actual environment
    # for china user, it could set `--set image.registry=nvcr.m.daocloud.io` to use a domestic registry
    $ helm install ofed-driver spiderchart/ofed-driver -n kube-system \
            --set image.OSName="ubuntu" \
            --set image.OSVer="22.04" \
            --set image.Arch="amd64"
    ```

    > 若希望 RDMA 系统工作在独占模式下，必须至少满足以下条件之一： (1） 基于 5.3.0 或更新版本的 Linux 内核，系统中加载的 RDMA 模块，rdma 核心包提供了在系统启动时自动加载相关模块的方法 (2） 需要 Mellanox OFED 4.7 版或更新版本。在这种情况下，不需要使用基于 5.3.0 或更新版本的内核。

2. 对于 SRIOV 场景，请设置主机上的 RDMA 子系统为 exclusive 模式，使得容器能够独立使用 RDMA 设备过程，避免与其他容器共享

    ```shell
    # Check the current operating mode (the Linux RDMA subsystem operates in shared mode by default):
    $ rdma system
       netns shared copy-on-fork on

    # Persist the exclusive mode to remain effective after a reboot
    $ echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

    # Switch the current operating mode to exclusive mode. If the setting fails, please reboot the host
    $ rdma system set netns exclusive

    # Verify the successful switch to exclusive mode
    $ rdma system
       netns exclusive copy-on-fork on
    ```

3. 设置网卡的 RDMA 工作模式（ Infiniband or ethernet ）

    3.1 确认网卡支持的工作模式：本示例环境中，宿主机上接入了 mellanox ConnectX 5 VPI 网卡，查询 RDMA 设备，确认网卡驱动安装完成

    ```shell
    $ rdma link
      link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
      link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1
      ....... 
    ```

    确认网卡的工作模式，如下输出表示网卡工作在 Ethernet 模式下，可实现 RoCE 通信

    ```shell
    $ ibstat mlx5_0 | grep "Link layer"
       Link layer: Ethernet
    ```

    如下输出表示网卡工作在 Infiniband 模式下，可实现 Infiniband 通信

    ```shell
    $ ibstat mlx5_0 | grep "Link layer"
       Link layer: InfiniBand
    ```

    如果网卡没有工作在预期的模式下，请输入如下命令，确认网卡支持配置 LINK_TYPE 参数，如果没有该参数，请更换支持的网卡型号

    ```shell
    $ mst start

    # check the card's PCIE 
    $ lspci -nn | grep Mellanox
          86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
          86:00.1 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
          ....... 

    # check whether the network card supports parameters LINK_TYPE 
    $ mlxconfig -d 86:00.0  q | grep LINK_TYPE
          LINK_TYPE_P1                                IB(1)
    ```

    3.2 批量设置网卡的工作模式：获取 [批量设置脚本](https://github.com/spidernet-io/spiderpool/blob/main/tools/scripts/setNicRdmaMode.sh)，按照如下设置后，请重启主机

    ```shell
    $ chmod +x ./setNicRdmaMode.sh

    # 批量查询所有 rdma 网卡工作在 ib 或者 eth 模式下
    $ ./setNicRdmaMode.sh q

    # 把所有 rdma 网卡切换到 eth 模式下
    $ RDMA_MODE="roce" ./setNicRdmaMode.sh

    # 把所有 rdma 网卡切换到 ib 模式下
    $ RDMA_MODE="infiniband" ./setNicRdmaMode.sh
    ```  

4. 为所有的 RDMA 网卡，设置 ip 地址、MTU 和 策略路由等

    > RDMA 场景下，通常交换机和主机网卡都会工作在较大的 MTU 参数下，以提高性能
    >
    > 因为 linux 主机默认只有一个缺省路由，在多网卡场景下，需要为不同网卡设置策略默认路由，以确保 hostnetwork 模式下的任务能正常运行 All-to-All 等通信
    >
    > 主机侧需要配置一条 RDMA 子网路由，以确保 RDMA 控制面流量能够正常传输

    获取 [ubuntu 网卡配置脚本](https://github.com/spidernet-io/spiderpool/blob/main/tools/scripts/setNicAddr.sh)，执行如下参考命令
    
    ```shell
    $ chmod +x ./setNicAddr.sh

    # 设置网卡并为首张 RDMA 网卡配置 RDMA 子网路由
    $ INTERFACE="eno3np2" ENABLE_RDMA_DEFAULT_ROUTE="true" RDMA_SUBNET="172.16.0.0/16" IPV4_GATEWAY="172.16.0.1" ./setNicAddr.sh

    # 对于非首网卡，只设置网卡 ip、mtu 和策略路由
    $ INTERFACE="eno3np3" IPV4_IP="172.16.1.10/24"  IPV4_GATEWAY="172.16.1.1" \
          MTU="4200" ENABLE_POLICY_ROUTE="true" ./setNicAddr.sh

    # 查看网卡 ip 和 mtu
    $ ip a s eno3np2
      4: eno3np2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 4200 qdisc mq state UP group default qlen 1000
        link/ether 38:68:dd:59:44:4a brd ff:ff:ff:ff:ff:ff
        altname enp8s0f2np2
        inet 172.16.0.10/24 brd 172.16.0.255 scope global eno3np2
          valid_lft forever preferred_lft forever
        inet6 fe80::3a68:ddff:fe59:444a/64 scope link proto kernel_ll
          valid_lft forever preferred_lft forever 

    # 查看策略路由
    $ ip rule
    0:  from all lookup local
    32763:  from 172.16.0.10 lookup 152 proto static
    32766:  from all lookup main
    32767:  from all lookup default

    $ ip rou show table 152
    default via 172.16.0.1 dev eno3np2 proto static

    # 查看 RDMA 默认子网路由
    $ ip r | grep 172.16
    172.16.0.0/16 via 172.16.0.1 dev eno3np2
    ```  

5. 配置主机 RDMA 无损网络

    在高性能网络场景下，RDMA 网络对于丢包非常敏感，一旦发生丢包重传，性能会急剧下降。因此要使得 RDMA 网络性能不受影响，丢包率必须保证在 1e-05（十万分之一）以下，最好为零丢包。对于 Roce 网络，可通过 PFC + ECN 机制来保障网络传输过程不丢包。

    可参考 [配置 RDMA 无损网络](../../roce-qos-zh_CN.md)

    > 配置无损网络要求必须在 RDMA Roce 网络环境下，不能是 Infiniband
    > 配置无损网络必须要求交换机支持 PFC + ECN 机制，并且配置与主机侧对齐，否则不能工作
    > 无损网络具体配置视环境确定，不同交换机厂商的配置方式不同

6. 开启 [GPUDirect RMDA](https://docs.nvidia.com/cuda/gpudirect-rdma/) 功能

    在安装或使用 [gpu-operator](https://github.com/NVIDIA/gpu-operator) 过程中

    a. 开启 helm 安装选项: `--set driver.rdma.enabled=true --set driver.rdma.useHostMofed=true`，gpu-operator 会安装 [nvidia-peermem](https://network.nvidia.com/products/GPUDirect-RDMA/) 内核模块，启用 GPUDirect RMDA 功能，加速 GPU 和 RDMA 网卡之间的转发性能。可在主机上输入如下命令，确认安装成功的内核模块

    ```shell
    $ lsmod | grep nvidia_peermem
      nvidia_peermem         16384  0
    ```

    b. 开启 helm 安装选项: `--set gdrcopy.enabled=true`，gpu-operator 会安装 [gdrcopy](https://developer.nvidia.com/gdrcopy) 内核模块，加速 GPU 显存 和 CPU 内存 之间的转发性能。可在主机上输入如下命令，确认安装成功的内核模块

    ```shell
    $ lsmod | grep gdrdrv
      gdrdrv                 24576  0
    ```

## 安装配置 Spiderpool

安装配置 Sriov 组件可参考 [AI-Cluster With SR-IOV](./get-started-sriov-zh_CN.md), 以下是配置 IP 池：

1. 配置 Docker 运行时支持（如果使用 Docker）

如果集群使用 Docker 作为容器运行时，需要为 spiderpool-agent 配置 `hostPID: true`：

```shell
kubectl patch daemonset spiderpool-agent -n spiderpool -p '{"spec":{"template":{"spec":{"hostPID":true}}}}'
```

2. 创建 IP 池，为每个节点首张 RDMA 网卡并配置 RDMA 子网默认路由：

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

依次为所有节点的每一个 RDMA 网卡创建 IP 池，，如节点 1 的其它 7 个轨道的 IP 池：


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

4. 创建支持子网自动匹配的 SpiderMultusConfig，以为 eno3np1 网卡为例：

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

**注意**:

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
        [{"name":"sriov-eno3np1","namespace":"spiderpool"},
        {"name":"sriov-eno3np2","namespace":"spiderpool"},
        {"name":"sriov-eno3np3","namespace":"spiderpool"},
        {"name":"sriov-eno3np4","namespace":"spiderpool"},
        {"name":"sriov-eno3np5","namespace":"spiderpool"},
        {"name":"sriov-eno3np6","namespace":"spiderpool"},
        {"name":"sriov-eno3np7","namespace":"spiderpool"},
        {"name":"sriov-eno3np8","namespace":"spiderpool"}]

    # sriov resource
    resources:
      limits:
        spidernet.io/eno3np1: 1
        spidernet.io/eno3np2: 1
        spidernet.io/eno3np3: 1
        spidernet.io/eno3np4: 1
        spidernet.io/eno3np5: 1
        spidernet.io/eno3np6: 1
        spidernet.io/eno3np7: 1
        spidernet.io/eno3np8: 1
        #nvidia.com/gpu: 1
    EOF

    $ helm install rdma-tools spiderchart/rdma-tools -f ./values.yaml
    
    ```

    在容器的网络命名空间创建过程中，Spiderpool 会对 sriov 接口上的网关进行连通性测试，如果如上应用的所有 POD 都启动成功，说明了每个节点上的 VF 设备的连通性成功，可进行正常的 RDMA 通信。

    <a id="checking-pod-network"></a>

2. 查看容器的网络命名空间状态

    可进入任一一个 POD 的网络命名空间中，确认具备 9 个网卡, 并且检查 Pod net1-net8 的 IP 地址是否属于该节点的 eno3np1-eno3np8 的 IP 地址范围：

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
           inet 172.16.1.10/27 brd 10.1.19.255 scope global net1
              valid_lft forever preferred_lft forever
           inet6 fe80::3897:49ff:fe35:7995/64 scope link
              valid_lft forever preferred_lft forever
       239: net2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP group default qlen 1000
           link/ether 1e:b6:13:0e:2a:d5 brd ff:ff:ff:ff:ff:ff
           inet 172.16.1.40/27 brd 10.1.19.255 scope global net1
              valid_lft forever preferred_lft forever
           inet6 fe80::1cb6:13ff:fe0e:2ad5/64 scope link
              valid_lft forever preferred_lft forever
       .....
    ```

    查看路由配置，Spiderpool 会自动为每个网卡调谐策略路由，确保每个网卡上收到的外部请求都会从该网卡上返回回复流量:

    ```shell
    root@rdma-tools-4v8t8:/# ip rule
    0:  from all lookup local
    32762:  from 172.16.11.230 lookup 107
    32763:  from 172.16.12.200 lookup 106
    32764:  from 172.16.13.176 lookup 105
    32765:  from 172.16.14.138 lookup 104
    32765:  from 172.16.15.106 lookup 103
    32765:  from 172.16.1.74 lookup 102
    32765:  from 172.16.1.42 lookup 101
    32765:  from 172.16.1.10 lookup 100
    32766:  from all lookup main
    32767:  from all lookup default

    root@rdma-tools-4v8t8:/# ip route show table 100
        default via 172.16.1.1 dev net1
    ```

    main 路由中，确保了 calico 网络流量、ClusterIP 流量、本地宿主机通信等流量都会从 calico 网卡转发，并且检查 RDMA 子网路由，确保 Pod 访问 RDMA 控制面流量从第一张 RDMA 网卡发出（net1):

    ```shell
    root@rdma-tools-4v8t8:/# ip r show table main
        default via 169.254.1.1 dev eth0
        172.16.0.0/16 via 172.16.1.1 dev net1
        172.16.1.0/27 dev net1 proto kernel scope link src 172.16.1.10
        172.16.1.32/27 dev net2 proto kernel scope link src 172.16.1.42
        172.16.1.64/27 dev net3 proto kernel scope link src 172.16.1.74
        172.16.1.96/27 dev net4 proto kernel scope link src 172.16.1.106
        172.16.1.128/27 dev net5 proto kernel scope link src 172.16.1.138
        172.16.1.160/27 dev net6 proto kernel scope link src 172.16.1.176
        172.16.1.192/27 dev net7 proto kernel scope link src 172.16.1.200
        172.16.1.224/27 dev net8 proto kernel scope link src 172.16.1.234
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

## 总结

基于主机 RDMA 轨道子网自动分配匹配的 IP 池功能为大规模 RDMA Zone 场景提供了优雅的解决方案,可以大大简化大规模 RDMA Zone 的网络配置和管理，提高运维效率和系统可靠性。
