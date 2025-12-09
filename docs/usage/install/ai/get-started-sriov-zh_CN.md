# AI Cluster With SR-IOV

**简体中文** | [**English**](./get-started-sriov.md)

## 介绍

本节介绍在建设 AI 集群场景下，如何基于 SR-IOV 技术给容器提供 RDMA 通信能力，它适用在 RoCE 和 Infiniband 网络场景下。

Spiderpool 使用了 [sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator) 为容器提供了基于 SR-IOV 接口的 RDMA 设备：

- Linux 的 RDMA 子系统，可两种在共享模式或独占模式下：

    1. 共享模式，容器中会看到 PF 接口的所有 VF 设备的 RDMA 设备，但只有分配给本容器的 VF 才具备从 0 开始的 GID Index。

    2. 独占模式，容器中只会看到分配给自身 VF 的 RDMA 设备，不会看见 PF 和 其它 VF 的 RDMA 设备。

- 在不同的网络场景下，使用了不同的 CNI

    1. Infiniband 网络场景下，使用 [IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 给 POD 提供 SR-IOV 网卡。

    2. RoCE 网络场景下， 使用了 [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) 来暴露宿主机上的 RDMA 网卡给 Pod 使用，暴露 RDMA 资源。可额外使用 [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni) 来完成 RDMA 设备隔离。

注意：

- 基于 SR-IOV 技术给容器提供 RDMA 通信能力只适用于裸金属环境，不适用于虚拟机环境。

## 对比 Macvlan CNI 的 RDMA 方案

| 比较维度     | Macvlan 共享 RDMA 方案                  | SR-IOV CNI 隔离 RDMA 方案          |
| ------------| ------------------------------------- | --------------------------------- |
| 网络隔离      | 所有容器共享 RDMA 设备，隔离性较差        | 容器独享 RDMA 设备，隔离性较好        |
| 性能         | 性能较高                               | 硬件直通，性能最优                   |
| 资源利用率    | 资源利用率较高                          | 较低，受硬件支持的 VFs 数量限制       |
| 配置复杂度    | 配置相对简单                            | 配置较为复杂，需要硬件支持和配置       |
| 兼容性       | 兼容性较好，适用于大多数环境               | 依赖硬件支持，兼容性较差              |
| 适用场景      | 适用于大多数场景，包括裸金属，虚拟机等      | 只适用于裸金属，不适用于虚拟机场景      |
| 成本         | 成本较低，因为不需要额外的硬件支持          | 成本较高，需要支持 SR-IOV 的硬件设备   |
| 支持 RDMA 协议 | 支持 Roce 协议，不支持 Infiniband 协议   | 支持 Roce 和 Infiniband 协议        |

## 共享子网组网方案

本文将以如下典型的 AI 集群拓扑为例，介绍如何搭建 Spiderpool

![AI Cluster](../../../images/ai-cluster.png)
图1 AI 集群拓扑

集群的网络规划如下：

1. 在节点的 eth0 网卡上运行 calico CNI，来承载 kubernetes 流量。AI workload 将会被分配一个 calico 的缺省网卡，进行控制面通信。

2. 节点上使用具备 RDMA 功能的 Mellanox ConnectX5 网卡来承载 AI 计算的 RDMA 流量，网卡接入到 rail optimized 网络中。AI workload 将会被额外分配所有 RDMA 网卡的 SR-IOV 虚拟化接口，确保 GPU 的高速网络通信。

## 独享子网组网方案

方案一适用于 AI 集群中所有节点的相同轨道网卡使用相同子网的场景。此种场景下，整个 AI 集群需要多个子网，比如如果节点拥有 8 个轨道的网卡，那么需要 8 个独立的子网。 在一些大规模 AI 集群中由于 IP 地址资源的限制，并不能提供这么多的子网。只能将有限的子网拆分给不同节点的不同轨道网卡使用，所以在此场景下，不同节点的相同轨道网卡往往被分配到不同的子网中。

假设我们拥有一个 16 位掩码的地址 IP 网段，每个节点每个轨道的网卡可有 32 个地址，则掩码为 27 位。在每个节点拥有 8 个轨道网卡，我们最多可支持 256 个节点（每个节点 8 个轨道网卡，每个轨道网卡 32 个地址）。

比如节点 node1 的 1 号轨道网卡可能使用 172.16.1.0/27 子网，而节点 node2 的 1 号轨道网卡可能使用 172.16.1.32/27 子网。

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
    如果未安装，可参考 [官方文档](https://docs.tigera.io/calico/latest/getting-started/kubernetes/) 或参考以下命令安装：

    ```shell
    $ kubectl apply -f https://github.com/projectcalico/calico/blob/master/manifests/calico.yaml
    $ kubectl wait --for=condition=ready -l k8s-app=calico-node  pod -n kube-system 
    # set calico to work on host eth0 
    $ kubectl set env daemonset -n kube-system calico-node IP_AUTODETECTION_METHOD=kubernetes-internal-ip
    # set calico to work on host eth0 
    $ kubectl set env daemonset -n kube-system calico-node IP6_AUTODETECTION_METHOD=kubernetes-internal-ip  
    ```

- 对于[组网方案二](#独享子网组网方案)，如果集群使用 Docker 作为容器运行时，需要为 spiderpool-agent 配置 `hostPID: true`，这样 Spiderpool-agent 能够获取到 Pod 的网络命名空间，使 Pod 能够基于主机网卡的网段分配对应的 IP 地址。

```shell
kubectl patch daemonset spiderpool-agent -n spiderpool -p '{"spec":{"template":{"spec":{"hostPID":true}}}}'
```

## 主机准备

1. 安装 RDMA 网卡驱动，然后重启主机（这样才能看到网卡）

    传统基于 MLNX_OFED 的驱动安装方式自 2024 年 10 月起已经停止维护，未来将被移除。所有新功能都会迁移到 [NVIDIA-DOCA](https://docs.nvidia.com/networking/dpu-doca/index.html#doca) 中, 推荐使用 NVIDIA DOCA-OFED 方式安装:

    前往 [NVIDIA-DOCA 下载页面](https://developer.nvidia.com/doca-downloads) 获取主机系统对应的 DOCA 版本, 如对于 Ubuntu 22.04 系统:

    ```shell
    sudo wget https://www.mellanox.com/downloads/DOCA/DOCA_v3.1.0/host/doca-host_3.1.0-091000-25.07-ubuntu2204_amd64.deb
    sudo dpkg -i doca-host_3.1.0-091000-25.07-ubuntu2204_amd64.deb
    sudo apt-get update
    sudo apt-get -y install doca-ofed
    ```

    对于 Mellanox 网卡，也可基于容器化安装驱动，实现对集群主机上所有 Mellanox 网卡批量安装驱动，运行如下命令，注意的是，该运行过程中需要访问因特网获取一些安装包。当所有的 ofed pod 进入 ready 状态，表示主机上已经完成了 OFED driver 安装。

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

    # 1. 如果配置所有网卡为 infiniband 或 roce 模式
    # 把所有 rdma 网卡切换到 eth 模式下
    $ RDMA_MODE="roce" ./setNicRdmaMode.sh

    # 把所有 rdma 网卡切换到 ib 模式下
    $ RDMA_MODE="infiniband" ./setNicRdmaMode.sh

    # 2. 如果需要为 GPU 和非 GPU 网卡分别配置不同模式
    # 基于 nvidia-smi 自动查询并配置 GPU 网卡的模式
    $ GPU_RDMA_MODE="infiniband" ./setNicRdmaMode.sh

    # 基于 nvidia-smi 自动查询并配置 GPU 和非 GPU 亲和的网卡
    $ GPU_RDMA_MODE="infiniband" OTHER_RDMA_MODE="roce" ./setNicRdmaMode.sh
    ```  

4. 为所有的 RDMA 网卡，设置 ip 地址、MTU 和 策略路由等

    > RDMA 场景下，通常交换机和主机网卡都会工作在较大的 MTU 参数下，以提高性能
    >
    > 因为 linux 主机默认只有一个缺省路由，在多网卡场景下，需要为不同网卡设置策略默认路由，以确保 hostnetwork 模式下的任务能正常运行 All-to-All 等通信
    >
    > 对于方案二：主机侧需要配置一条 RDMA 子网路由，以确保 RDMA 控制面流量能够正常传输

    获取 [ubuntu 网卡配置脚本](https://github.com/spidernet-io/spiderpool/blob/main/tools/scripts/setNicAddr.sh)，执行如下参考命令
    
    ```shell
    $ chmod +x ./setNicAddr.sh

    # 对于共享子网组网方案，设置网卡
    $ INTERFACE="eno3np2" IPV4_IP="172.16.0.10/24"  IPV4_GATEWAY="172.16.0.1" \
          MTU="4200" ENABLE_POLICY_ROUTE="true" ./setNicAddr.sh

    # 对于独享子网组网方案，必须选择一张网卡（一般为节点的 1 号轨道的 RDMA 网卡）作为 RDMA 子网路由的网卡，需要设置：
    ENABLE_RDMA_DEFAULT_ROUTE="true" RDMA_SUBNET="172.16.0.0/16"，对于其它网卡不需要配置 RDMA 子网路由
    # ENABLE_RDMA_DEFAULT_ROUTE="true" 表示该网卡将作为 RDMA 子网路由的网卡
    # RDMA_SUBNET="172.16.0.0/16" 表示 RDMA 网络的子网
    $ INTERFACE="eno3np2" ENABLE_RDMA_DEFAULT_ROUTE="true" RDMA_SUBNET="172.16.0.0/16" IPV4_GATEWAY="172.16.0.1" ./setNicAddr.sh

    # 以 eno3np2 为例，查看网卡 ip 和 mtu 是否预期设置
    $ ip a s eno3np2
      4: eno3np2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 4200 qdisc mq state UP group default qlen 1000
        link/ether 38:68:dd:59:44:4a brd ff:ff:ff:ff:ff:ff
        altname enp8s0f2np2
        inet 172.16.0.10/24 brd 172.16.0.255 scope global eno3np2
          valid_lft forever preferred_lft forever
        inet6 fe80::3a68:ddff:fe59:444a/64 scope link proto kernel_ll
          valid_lft forever preferred_lft forever 

    # 检查当前网卡是否正确设置策略路由: 确保从该网卡发出的流量需要从各自的策略路由表项转发：
    $ ip rule
    0:  from all lookup local
    32763:  from 172.16.0.10 lookup 152 proto static
    32766:  from all lookup main
    32767:  from all lookup default

    $ ip rou show table 152
    default via 172.16.0.1 dev eno3np2 proto static 

    # 对于方案二，当设置 ENABLE_RDMA_DEFAULT_ROUTE="true" RDMA_SUBNET="172.16.0.0/16"，需要确保 RDMA 子网路由是否配置成功，并从当前网卡转发：
    $ ip r
    ...
    172.16.0.0/16 via 172.16.0.1 dev eno3np2
    ...
    ```

    对于两种方案： 多 RDMA 网卡时可能访问集群流量的数据包（比如 Calico）的源 IP 被 Linux 随机选择到了 RDMA 网卡的 IP 地址上，导致访问失败。 所以我们需要保证非 RDMA 流量不需要从 RDMA 网卡转发：

    获取[配置脚本](https://github.com/spidernet-io/spiderpool/blob/main/tools/scripts/setRdmaRule.sh), 执行以下命令:

    ```shell
    $ chmod +x ./setRdmaRule.sh
    $ RDMA_CIDR=172.16.0.0/16 ./setRdmaRule.sh
    ```

    > RDMA_CIDR 表示 RDMA 网络的子网

    执行完成后检查 ip rule 是否预期设置:

    ```shell
    $ ip rule
    0:  from all lookup local
    32762:  not to 172.16.0.0/16 lookup main proto static
    32763:  from 172.16.0.10 lookup 152 proto static
    32766:  from all lookup main
    32767:  from all lookup default
    ```

5. 配置主机 RDMA 无损网络

    在高性能网络场景下，RDMA 网络对于丢包非常敏感，一旦发生丢包重传，性能会急剧下降。因此要使得 RDMA 网络性能不受影响，丢包率必须保证在 1e-05（十万分之一）以下，最好为零丢包。对于 Roce 网络，可通过 PFC + ECN 机制来保障网络传输过程不丢包。

    可参考 [配置 RDMA 无损网络](../../roce-qos-zh_CN.md)

    > 配置无损网络要求必须在 RDMA Roce 网络环境下，不能是 Infiniband
    > 配置无损网络必须要求交换机支持 PFC + ECN 机制，并且配置与主机侧对齐，否则不能工作

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

    c. 禁用 PCI 访问控制服务 (ACS) 以支持 GPUDirect RDMA

    ```shell
    sudo lspci -vvv | grep ACSCtl  
    ```
    
    如果输出显示 "SrcValid+", 表示 ACS 可能被启用。为了使 GPUDirect RDMA 正常工作，需要禁用 PCI 访问控制服务 (ACS)。可通过 BIOS 禁用 IO 虚拟化或 VT-d 来实现。对于 Broadcom PLX 设备，也可以通过操作系统禁用，但需要在每次重启后重新执行。

    更多信息请参考 [NVIDIA NCCL 故障排除指南](https://docs.nvidia.com/deeplearning/nccl/user-guide/docs/troubleshooting.html#pci-access-control-services-acs)

## 安装 Spiderpool

1. 使用 helm 安装 Spiderpool，并启用 SR-IOV 组件

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set sriov.install=true
    ```

    > - 如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。
    > - 设置 `--set spiderpoolAgent.prometheus.enabled --set spiderpoolAgent.prometheus.enabledRdmaMetric=true` 和 `--set grafanaDashboard.install=true` 命令行参数可以开启 RDMA metrics exporter 和 Grafana dashboard，更多可以查看 [RDMA metrics](../../rdma-metrics.md).

    完成后，安装的组件如下

    ```shell
    $ kubectl get pod -n spiderpool
        operator-webhook-sgkxp                         1/1     Running     0          1m
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        sriov-network-config-daemon-8h576              1/1     Running     0          1m
        sriov-network-config-daemon-n629x              1/1     Running     0          1m
    ```

2. 配置 SR-IOV operator, 在每个主机上创建出 VF 设备

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
    # 对于 ethernet 网络，设置 LINK_TYPE=eth， 对于 Infiniband 网络，设置 LINK_TYPE=ib
    $ LINK_TYPE=eth NIC_NAME=eno3np2 VF_NUM=12
    $ cat <<EOF | kubectl apply -f -
    apiVersion: sriovnetwork.openshift.io/v1
    kind: SriovNetworkNodePolicy
    metadata:
      name: gpu1-nic-policy
      namespace: spiderpool
    spec:
      nodeSelector:
        kubernetes.io/os: "linux"
      resourceName: eno3np2
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

3. 创建 CNI 配置和对应的 ippool 资源

    3.1 由于[共享子网组网方案](#共享子网组网方案)，所有节点的相同轨道网卡使用相同的子网，因此，只需要为每个节点的相同轨道网卡创建一个 IP 池即可：

    (1) 对于 Infiniband 网络，请为所有的 GPU 亲和的 SR-IOV 网卡配置 [IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 配置，并创建对应的 IP 地址池 。 如下例子，配置了 GPU1 亲和的网卡和 IP 地址池

    ```shell
    $ cat <<EOF | kubectl apply -f -
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
    spec:
      cniType: ib-sriov
      ibsriov:
        resourceName: spidernet.io/eno3np2
        rdmaIsolation: true
        ippools:
          ipv4: ["gpu1-net91"]
    EOF
    ```

    如果您需要自定义配置 VF 的 MTU，参考 [自定义配置 VF 的 MTU](#自定义-vf-的-mtu).

    (2) 对于 Ethernet 网络，请为所有的 GPU 亲和的 SR-IOV 网卡配置 [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) 配置，并创建对应的 IP 地址池 。 如下例子，配置了 GPU1 亲和的网卡和 IP 地址池

    ```shell
    $ cat <<EOF | kubectl apply -f -
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
    spec:
      cniType: sriov
      sriov:
        resourceName: spidernet.io/eno3np2
        enableRdma: true
        ippools:
          ipv4: ["gpu1-net11"]
    EOF
    ```

    如果您需要自定义配置 VF 的 MTU，参考 [自定义配置 VF 的 MTU](#自定义-vf-的-mtu).

    3.2 对于[独享子网组网方案](#独享子网组网方案)，创建 CNI 配置和对应的 ippool 资源

    该方案下，每个节点的相同轨道网卡使用不同的子网，因此，需要为每个节点的每一个轨道网卡创建独立的 IP 池资源，这样 Spiderpool 可以保证当 Pod 调度到节点上时，能够基于 Pod 的节点 master 网卡所对应的 IP 池分配 IP。

    - 创建 IP 池，为每个节点第一张 RDMA 网卡（一般是 1 号轨道网卡）并配置 RDMA 子网路由：

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

    - 创建支持子网自动匹配的 SpiderMultusConfig，以 eno3np1 网卡为例：

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

    可进入任一一个 POD 的网络命名空间中，确认具备 9 个网卡, 对于[组网方案二](#独享子网组网方案)场景下，需要额外检查 Pod net1-net8 的 IP 地址是否属于该节点的 eno3np1-eno3np8 的 IP 地址范围，并且检查 main 路由表中是否具备 RDMA 子网路由：

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

## （可选）Infiniband 网络下对接 UFM

对于使用了 Infiniband 网络的集群，如果网络中有 [UFM 管理平台](https://www.nvidia.com/en-us/networking/infiniband/ufm/)，可使用 [ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) 插件，它以 daemonset 形式运行，监控所有使用 SRIOV 网卡的容器，把 VF 设备的 Pkey 和 GUID 上报给 UFM 。

1. 在 UFM 主机上创建通信所需要的证书：

    ```shell
    # replace to right address
    $ UFM_ADDRESS=172.16.10.10
    $ openssl req -x509 -newkey rsa:4096 -keyout ufm.key -out ufm.crt -days 365 -subj '/CN=${UFM_ADDRESS}'

    # Copy the certificate files to the UFM certificate directory:
    $ cp ufm.key /etc/pki/tls/private/ufmlocalhost.key
    $ cp ufm.crt /etc/pki/tls/certs/ufmlocalhost.crt

    # For containerized UFM deployment, restart the container service
    $ docker restart ufm

    # For host-based UFM deployment, restart the UFM service
    $ systemctl restart ufmd
    ```

2. 在 kubernetes 集群上，创建 ib-kubernetes 所需的通信证书。把 UFM 主机上生成的 ufm.crt 文件传输至 kubernetes 节点上，并使用如下命令创建证书

    ```shell
    # replace to right user
    $ UFM_USERNAME=admin

    # replace to right password
    $ UFM_PASSWORD=12345

    # replace to right address
    $ UFM_ADDRESS="172.16.10.10"
    $ kubectl create secret generic ib-kubernetes-ufm-secret --namespace="kube-system" \
                 --from-literal=UFM_USER="${UFM_USERNAME}" \
                 --from-literal=UFM_PASSWORD="${UFM_PASSWORD}" \
                 --from-literal=UFM_ADDRESS="${UFM_ADDRESS}" \
                 --from-file=UFM_CERTIFICATE=ufm.crt 
    ```

3. 在 kubernetes 集群上安装 ib-kubernetes

    ```shell
    git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
    $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
    kubectl create -f deployment/ib-kubernetes.yaml 
    ```

4. 在 Infiniband 网络下，创建 Spiderpool 的 SpiderMultusConfig 时，可配置 pkey，使用该配置创建的 POD 将生效 pkey 配置，且被 ib-kubernetes 同步给 UFM

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: ib-sriov
      namespace: spiderpool
    spec:
      cniType: ib-sriov
      ibsriov:
        pkey: 1000
        ...
    EOF
    ```

    > Note: Each node in an Infiniband Kubernetes deployment may be associated with up to 128 PKeys due to kernel limitation

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
