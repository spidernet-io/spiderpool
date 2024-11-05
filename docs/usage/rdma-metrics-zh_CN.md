# RDMA 指标

[**English**](./rdma-metrics.md) | **简体中文**

RDMA 是一种高效的网络通信技术，允许一台计算机直接访问另一台计算机的内存，无需操作系统介入，从而减少延迟，提高数据传输速度和效率。RDMA 支持高速数据传输，减少 CPU 负载，非常适用于需要高速网络通信的场景。

在 Kubernetes 集群中，spiderpool CNI 支持 RoCE 和 IB 2 种 RDMA 场景，Pod 可以通过共享和独占的方式使用 RDMA 网卡，用户可以根据需求选择合适的方式来使用 RDMA 网卡。

spiderpool 同时提供了 RDMA exporter 功能和 grafana 监控面板，通过实时监控 Pod/Node RDMA 网络的性能，包括吞吐量、延迟、丢包率等，可以时发现问题并采取措施解决，提高网络的可靠性和性能。

## RDMA 指标的常见场景

1. **性能监控**:
    - **吞吐量**: 测量通过网络传输的数据量。
    - **延迟**: 测量数据从源到目的地的传输时间。
    - **丢包率**: 监控传输过程中丢失的数据包数量。

2. **错误检测**:
    - **传输错误**: 检测数据传输中的错误。
    - **连接失败**: 监控失败的连接尝试和断开连接。

3. **网络健康状况**:
    - **拥塞**: 检测网络拥塞和瓶颈。

## 如何开启

```shell
helm upgrade --install spiderpool spiderpool/spiderpool --reuse-values --wait --namespace spiderpool --create-namespace \
  --set spiderpoolAgent.prometheus.enabled=true \
  --set spiderpoolAgent.prometheus.enabledRdmaMetric=true \
  --set grafanaDashboard.install=true \
  --set spiderpoolAgent.prometheus.serviceMonitor.install=true
```

- 通过设置 `--reuse-values` 重用现有的配置
- 通过设置 `--wait` 等待所有 Pod 运行
- 通过设置 `--namespace` 指定 Helm 安装的命名空间
- 通过设置 `--set spiderpoolAgent.prometheus.enabled` 启用 Prometheus 监控
- 通过设置 `--set spiderpoolAgent.prometheus.enabledRdmaMetric=true`，可以启用 RDMA 指标 exporter
- 通过设置 `--set grafanaDashboard.install=true`，可以启用 GrafanaDashboard CR 看板

## 指标列表

以下是经过整理后的表格，包含了"指标名称"、"指标类型"、"指标含义"和"备注"四列：

| 指标名称                            | 指标类型    | 指标含义                                        | 备注 |
|---------------------------------|---------|---------------------------------------------|----|
| rx_write_requests               | Counter | 接收到的写请求的数量                                  |    |
| rx_read_requests                | Counter | 接收到的读请求的数量                                  |    |
| rx_atomic_requests              | Counter | 接收到的原子请求的数量                                 |    |
| rx_dct_connect                  | Counter | 接收到的 DCT 连接请求的数量                            |    |
| out_of_buffer                   | Counter | 缓冲区不足错误的数量                                  |    |
| out_of_sequence                 | Counter | 收到的乱序包数量                                    |    |
| duplicate_request               | Counter | 重复请求的数量                                     |    |
| rnr_nak_retry_err               | Counter | 收到的 RNR NAK 包未超过 QP 重试限制的数量                 |    |
| packet_seq_err                  | Counter | 包序列错误的数量                                    |    |
| implied_nak_seq_err             | Counter | 隐含 NAK 序列错误的数量                              |    |
| local_ack_timeout_err           | Counter | 发送端 QP 的 ack 计时器过期的次数（适用于 RC, XRC, DCT QPs） |    |
| resp_local_length_error         | Counter | 响应者检测到本地长度错误的次数                             |    |
| resp_cqe_error                  | Counter | 响应 CQE 错误的数量                                |    |
| req_cqe_error                   | Counter | 请求者检测到 CQE 完成且带错误的次数                        |    |
| req_remote_invalid_request      | Counter | 请求者检测到远程无效请求错误的次数                           |    |
| req_remote_access_errors        | Counter | 请求的远程访问错误的数量                                |    |
| resp_remote_access_errors       | Counter | 响应的远程访问错误的数量                                |    |
| resp_cqe_flush_error            | Counter | 响应 CQE 刷新错误的数量                              |    |
| req_cqe_flush_error             | Counter | 请求 CQE 刷新错误的数量                              |    |
| roce_adp_retrans                | Counter | RoCE 自适应重传的次数                               |    |
| roce_adp_retrans_to             | Counter | RoCE 自适应重传超时的次数                             |    |
| roce_slow_restart               | Counter | RoCE 缓慢重启的次数                                |    |
| roce_slow_restart_cnps          | Counter | RoCE 缓慢重启产生的CNP包数                           |    |
| roce_slow_restart_trans         | Counter | RoCE 缓慢重启状态转换为缓慢重启的次数                       |    |
| rp_cnp_ignored                  | Counter | Reaction Point HCA 接收到并忽略的 CNP 包数量          |    |
| rp_cnp_handled                  | Counter | Reaction Point HCA 处理以降低传输速率的 CNP 包数量       |    |
| np_ecn_marked_roce_packets      | Counter | 进入 Pod/Node 的方向收到的 ECN，表示路径拥塞               |    |
| np_cnp_sent                     | Counter | 通知点在 RoCEv2 IP 头部注意到拥塞体验时发送的 CNP 包数         |    |
| rx_icrc_encapsulated            | Counter | 具有 ICRC 错误的 RoCE 包数量                        |    |
| rx_vport_rdma_unicast_packets   | Counter | 单播 RDMA 包数量                                 |    |
| tx_vport_rdma_unicast_packets   | Counter | 发送的单播 RDMA 包数量                              |    |
| rx_vport_rdma_multicast_packets | Counter | 接收到的多播 RDMA 包数量                             |    |
| tx_vport_rdma_multicast_packets | Counter | 发送的多播 RDMA 包数量                              |    |
| rx_vport_rdma_unicast_bytes     | Counter | 接收到的单播 RDMA 包的字节数                           |    |
| tx_vport_rdma_unicast_bytes     | Counter | 发送的单播 RDMA 包的字节数                            |    |
| rx_vport_rdma_multicast_bytes   | Counter | 接收到的多播 RDMA 包的字节数                           |    |
| tx_vport_rdma_multicast_bytes   | Counter | 发送的多播 RDMA 包的字节数                            |    |
| vport_speed_mbps                | 速度      | 端口的速度，以兆位每秒（Mbps）表示                         |    |
