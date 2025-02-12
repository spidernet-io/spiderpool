# Configure Lossless Network For RoCE

## Introduction

In various HPC high-performance computing scenarios, the main requirements for networks are high throughput and low latency. To achieve high throughput and low latency, the industry generally uses RDMA (Remote Direct Memory Access) to replace the TCP protocol. However, RDMA networks are very sensitive to packet loss. Once packet retransmission occurs, performance will drop sharply. Therefore, to ensure that RDMA throughput is not affected, the packet loss rate must be kept below 1e-05 (one in 100,000), ideally zero.

RoCE (RDMA over Converged Ethernet) networks use PFC+ECN features to ensure no packet loss during network transmission.

- PFC: Priority Flow Control, IEEE 802.1Qbb, flow control based on priority.
- ECN: Explicit Congestion Notification, implemented by setting flags in specific bits of the IP header to indicate network congestion without dropping packets.

This document will introduce how to configure a lossless network on the host side for RoCE. Note: This does not involve switch configuration.

## How to Configure

This document provides a script to help configure a lossless network on the host side using Systemd.

1. Download the script, then add script permissions and execute

    ```shell
    cd /usr/local/bin
    curl -O https://raw.githubusercontent.com/spidernet-io/spiderpool/master/docs/usage/rdma-qos.sh
    chmod +x rdma-qos.sh
    ```

    If the server is a GPU server, you need to configure the NIC list and the priority queues for RDMA traffic and CNP packets. Make sure the NIC names are consistent with the actual names.

    ```shell
    chmod +x rdma-qos.sh 
    GPU_NIC_LIST=eth0,eth1 GPU_RDMA_PRIORITY=5 GPU_CNP_PRIORITY=6 bash rdma-qos.sh
    ```

    - GPU_NIC_LIST: Specifies the list of NICs to configure.
    - GPU_RDMA_PRIORITY: Specifies the priority queue for Roce traffic, with a range of 0-7, default is 5.
    - GPU_CNP_PRIORITY: Specifies the priority queue for CNP packets, with a range of 0-7, default is 6.
    - GPU_RDMA_QOS: Specifies the DSCP for Roce traffic. Default is empty, calculated as GPU_RDMA_PRIORITY * 8 = 40.
    - GPU_CNP_QOS: Specifies the DSCP for CNP packets. Default is empty, calculated as GPU_CNP_PRIORITY * 8 = 48.

    If the server is a Storage server, you need to configure the NIC list and the priority queues for RDMA traffic and CNP packets. Make sure the NIC names are consistent with the actual names.

    ```shell
    chmod +x rdma-qos.sh 
    STORAGE_NIC_LIST=eth0,eth1 STORAGE_RDMA_PRIORITY=5 STORAGE_CNP_PRIORITY=6 bash rdma-qos.sh
    ```

    - STORAGE_NIC_LIST: Specifies the list of NICs to configure.
    - STORAGE_RDMA_PRIORITY: Specifies the priority queue for Roce traffic, with a range of 0-7, default is 5.
    - STORAGE_CNP_PRIORITY: Specifies the priority queue for CNP packets, with a range of 0-7, default is 6.
    - STORAGE_RDMA_QOS: Specifies the DSCP for Roce traffic. Default is empty, calculated as STORAGE_RDMA_PRIORITY * 8 = 40.
    - STORAGE_CNP_QOS: Specifies the DSCP for CNP packets. Default is empty, calculated as STORAGE_RDMA_PRIORITY * 8 = 48.

2. Check Systemd service running status

    ```shell
    systemctl status rdma-qos.service
    journalctl -u rdma-qos.service
    ```
