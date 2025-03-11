# 配置无损网络

## 介绍

在各种 HPC 高性能计算场景中，对网络的诉求基本上是高吞吐和低时延这两个重要特性，为了实现高吞吐和低时延，业界一般采用 RDMA(Remote Direct Memory Access，远程直接内存访问)替代 TCP 协议。但是 RDMA 网络对于丢包非常敏感，一旦发生丢包重传，性能会急剧下降。因此要使得 RDMA 吞吐不受影响，丢包率必须保证在 1e-05（十万分之一）以下，最好为丢包率为 0。

RoCE (RDMA over Converged Ethernet)网络通过 PFC+ECN 特性来保障网络传输过程不丢包。

- PFC: 优先级流量控制 (Priority Flow Control)，IEEE 802.1Qbb，基于优先级的流量控制。
- ECN: 显式拥塞通知（Explicit Congestion Notification），通过在 IP 头部的特定位上设置标志实现：在不丢弃数据包的情况下指示网络拥塞。

本文将介绍如何在主机侧配置 Roce 的无损网络。注： 不涉及交换机配置。

## 如何配置

本文档提供一个脚本以 Systemd 方式帮助配置主机侧的 Roce 无损网络。

1. 下载脚本到本地文件路径，并添加脚本权限执行:


    ```shell
    wget https://raw.githubusercontent.com/spidernet-io/spiderpool/refs/heads/main/docs/example/qos/rdma-qos.sh
    chmod +x rdma-qos.sh 
    ```

    如果是 GPU 服务器，需要配置网卡列表及 RDMA 流量 和 CNP 报文的优先级队列。

    ```shell
    GPU_NIC_LIST=all GPU_RDMA_PRIORITY=5 GPU_CNP_PRIORITY=6 bash rdma-qos.sh
    ```

    - GPU_NIC_LIST: 指定要配置的网卡列表。为 all 时表示配置所有 RDMA 网卡， 或可配置具体 RDMA 网卡名称，如 eth0, eth1。
    - GPU_RDMA_PRIORITY: 指定 Roce 流量的优先级队列，配置范围为 0~7，默认为 5。
    - GPU_CNP_PRIORITY: 指定 CNP 报文的优先级队列，配置范围为 0~7，默认为 6。
    - GPU_RDMA_QOS: 指定 Roce 流量的 dscp。默认为空，默认值 = GPU_RDMA_PRIORITY * 8 = 40。
    - GPU_CNP_QOS: 指定 CNP 报文的 dscp。默认为空，默认值 = GPU_CNP_PRIORITY * 8 = 48。

    如果是 Storage 服务器，需要配置网卡列表及 RDMA 流量 和 CNP 报文的优先级队列。注意网卡名称与实际保持一致。

    ```shell
    STORAGE_NIC_LIST=all STORAGE_RDMA_PRIORITY=5 STORAGE_CNP_PRIORITY=6 bash rdma-qos.sh
    ```

    - STORAGE_NIC_LIST: 指定要配置的网卡列表。为 all 时表示配置所有 RDMA 网卡， 或可配置具体 RDMA 网卡名称，如 eth0, eth1。
    - STORAGE_RDMA_PRIORITY: 指定 Roce 流量的优先级队列，配置范围为 0~7，默认为 5。
    - STORAGE_CNP_PRIORITY: 指定 CNP 报文的优先级队列，配置范围为 0~7，默认为 6。
    - STORAGE_RDMA_QOS: 指定 Roce 流量的 dscp。默认为空，默认值 = STORAGE_RDMA_PRIORITY * 8 = 40。
    - STORAGE_CNP_QOS: 指定 CNP 报文的 dscp。默认为空，默认值 = STORAGE_RDMA_PRIORITY * 8 = 48。

    通过同时需要在一台服务器同时设置 GPU 或 Storage 网卡，可使用以下命令进行配置:

    ```shell
    GPU_NIC_LIST="eth1 eth2"  GPU_RDMA_PRIORITY=5  GPU_CNP_PRIORITY=6  \
    STORAGE_NIC_LIST="eno1 eno2" STORAGE_RDMA_PRIORITY=2  STORAGE_CNP_PRIORITY=3 \
    ./rdma-qos.sh   
    ```

2. 检查执行结果，查看 Systemd 服务运行状态

    执行完毕后，可通过 `rdma-qos.sh q` 查询配置结果, 是否符合预期。

    ```shell
    ./set-rdma-qos.sh q
    ======== show configuration for device eth0 / mlx5_0========
    Priority trust state: dscp
    PFC configuration:
            priority    0   1   2   3   4   5   6   7
            enabled     0   0   0   0   0   1   0   0   
            buffer      0   0   0   0   0   1   0   0   
    ECN Enabled for priority 0: /sys/class/net/eth0/ecn/roce_np/enable/0 = 1
    ECN Enabled for priority 0: /sys/class/net/eth0/ecn/roce_rp/enable/0 = 1
    ECN Enabled for priority 1: /sys/class/net/eth0/ecn/roce_np/enable/1 = 1
    ECN Enabled for priority 1: /sys/class/net/eth0/ecn/roce_rp/enable/1 = 1
    ECN Enabled for priority 2: /sys/class/net/eth0/ecn/roce_np/enable/2 = 1
    ECN Enabled for priority 2: /sys/class/net/eth0/ecn/roce_rp/enable/2 = 1
    ECN Enabled for priority 3: /sys/class/net/eth0/ecn/roce_np/enable/3 = 1
    ECN Enabled for priority 3: /sys/class/net/eth0/ecn/roce_rp/enable/3 = 1
    ECN Enabled for priority 4: /sys/class/net/eth0/ecn/roce_np/enable/4 = 1
    ECN Enabled for priority 4: /sys/class/net/eth0/ecn/roce_rp/enable/4 = 1
    ECN Enabled for priority 5: /sys/class/net/eth0/ecn/roce_np/enable/5 = 1
    ECN Enabled for priority 5: /sys/class/net/eth0/ecn/roce_rp/enable/5 = 1
    ECN Enabled for priority 6: /sys/class/net/eth0/ecn/roce_np/enable/6 = 1
    ECN Enabled for priority 6: /sys/class/net/eth0/ecn/roce_rp/enable/6 = 1
    ECN Enabled for priority 7: /sys/class/net/eth0/ecn/roce_np/enable/7 = 1
    ECN Enabled for priority 7: /sys/class/net/eth0/ecn/roce_rp/enable/7 = 1
    QOS for CNP: /sys/class/net/eth0/ecn/roce_np/cnp_dscp = 48
    cma_roce_tos: 160
    QOS for rdma: /sys/class/infiniband/mlx5_0/tc/1/traffic_class = Global tclass=160
    ```

    检查 Systedm 运行状态：
    
    ```shell
    systemctl status rdma-qos.service
    journalctl -u rdma-qos.service
    ```
