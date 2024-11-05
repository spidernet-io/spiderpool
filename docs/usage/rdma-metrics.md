# RDMA Metrics

**English** ｜ [**简体中文**](./rdma-metrics-zh_CN.md)

RDMA is an efficient network communication technology that allows one computer to directly access the memory of another computer without involving the operating system, thus reducing latency and improving data transfer speed and efficiency. RDMA supports high-speed data transmission and reduces CPU load, making it ideal for scenarios requiring high-speed network communication.

In a Kubernetes cluster, the spiderpool CNI supports two RDMA scenarios: RoCE and IB. Pods can use the RDMA network card in either shared or exclusive modes. Users can choose the appropriate method based on their needs for utilizing RDMA network cards.

Spiderpool also provides an RDMA exporter feature and a Grafana monitoring panel. By monitoring the performance of Pod/Node RDMA networks in real-time, including throughput, latency, packet loss rate, etc., issues can be detected and measures taken to improve network reliability and performance.

## Common Scenarios for RDMA Metrics

1. **Performance Monitoring**:
    - **Throughput**: Measures the amount of data transmitted over the network.
    - **Latency**: Measures the time it takes for data to travel from source to destination.
    - **Packet Loss Rate**: Monitors the number of data packets lost during transmission.

2. **Error Detection**:
    - **Transmission Errors**: Detects errors in data transmission.
    - **Connection Failures**: Monitors failed connection attempts and disconnects.

3. **Network Health**:
    - **Congestion**: Detects network congestion and bottlenecks.

## How to Enable

```shell
helm upgrade --install spiderpool spiderpool/spiderpool --reuse-values --wait --namespace spiderpool --create-namespace \
  --set spiderpoolAgent.prometheus.enabled=true \
  --set spiderpoolAgent.prometheus.enabledRdmaMetric=true \
  --set grafanaDashboard.install=true \
  --set spiderpoolAgent.prometheus.serviceMonitor.install=true
```

- Use `--reuse-values` to reuse existing configurations.
- Use `--wait` to wait for all Pods to be running.
- Use `--namespace` to specify the Helm installation namespace.
- Use `--set spiderpoolAgent.prometheus.enabled` to enable Prometheus monitoring.
- Use `--set spiderpoolAgent.prometheus.enabledRdmaMetric=true` to enable the RDMA metric exporter.
- Use `--set grafanaDashboard.install=true` to enable GrafanaDashboard CR.

## Metrics List

Below is a table containing "Metric Name," "Metric Type," "Metric Meaning," and "Remarks":

| Metric Name                     | Type    | Meaning                                                                         | Remarks               |
|---------------------------------|---------|---------------------------------------------------------------------------------|-----------------------|
| rx_write_requests               | Counter | Number of received write requests                                               |                       |
| rx_read_requests                | Counter | Number of received read requests                                                |                       |
| rx_atomic_requests              | Counter | Number of received atomic requests                                              |                       |
| rx_dct_connect                  | Counter | Number of received DCT connection requests                                      |                       |
| out_of_buffer                   | Counter | Number of buffer insufficiency errors                                           |                       |
| out_of_sequence                 | Counter | Number of out-of-sequence packets received                                      |                       |
| duplicate_request               | Counter | Number of duplicate requests                                                    |                       |
| rnr_nak_retry_err               | Counter | Count of RNR NAK packets not exceeding QP retry limit                           |                       |
| packet_seq_err                  | Counter | Number of packet sequence errors                                                |                       |
| implied_nak_seq_err             | Counter | Number of implied NAK sequence errors                                           |                       |
| local_ack_timeout_err           | Counter | Number of times the sender's QP ack timer expired                               | RC, XRC, DCT QPs only |
| resp_local_length_error         | Counter | Number of times a respondent detected a local length error                      |                       |
| resp_cqe_error                  | Counter | Number of response CQE errors                                                   |                       |
| req_cqe_error                   | Counter | Number of times a requester detected CQE completion with errors                 |                       |
| req_remote_invalid_request      | Counter | Number of remote invalid request errors detected by requester                   |                       |
| req_remote_access_errors        | Counter | Number of requested remote access errors                                        |                       |
| resp_remote_access_errors       | Counter | Number of response remote access errors                                         |                       |
| resp_cqe_flush_error            | Counter | Number of response CQE flush errors                                             |                       |
| req_cqe_flush_error             | Counter | Number of request CQE flush errors                                              |                       |
| roce_adp_retrans                | Counter | Number of RoCE adaptive retransmissions                                         |                       |
| roce_adp_retrans_to             | Counter | Number of RoCE adaptive retransmission timeouts                                 |                       |
| roce_slow_restart               | Counter | Number of RoCE slow restarts                                                    |                       |
| roce_slow_restart_cnps          | Counter | Number of CNP packets generated during RoCE slow restart                        |                       |
| roce_slow_restart_trans         | Counter | Number of times state transitioned to slow restart                              |                       |
| rp_cnp_ignored                  | Counter | Number of CNP packets received and ignored by Reaction Point HCA                |                       |
| rp_cnp_handled                  | Counter | Number of CNP packets handled by Reaction Point HCA to reduce transmission rate |                       |
| np_ecn_marked_roce_packets      | Counter | Number of ECN-marked RoCE packets indicating path congestion                    |                       |
| np_cnp_sent                     | Counter | Number of CNP packets sent when congestion is experienced in RoCEv2 IP header   |                       |
| rx_icrc_encapsulated            | Counter | Number of RoCE packets with ICRC errors                                         |                       |
| rx_vport_rdma_unicast_packets   | Counter | Number of received unicast RDMA packets                                         |                       |
| tx_vport_rdma_unicast_packets   | Counter | Number of transmitted unicast RDMA packets                                      |                       |
| rx_vport_rdma_multicast_packets | Counter | Number of received multicast RDMA packets                                       |                       |
| tx_vport_rdma_multicast_packets | Counter | Number of transmitted multicast RDMA packets                                    |                       |
| rx_vport_rdma_unicast_bytes     | Counter | Number of bytes received in unicast RDMA packets                                |                       |
| tx_vport_rdma_unicast_bytes     | Counter | Number of bytes transmitted in unicast RDMA packets                             |                       |
| rx_vport_rdma_multicast_bytes   | Counter | Number of bytes received in multicast RDMA packets                              |                       |
| tx_vport_rdma_multicast_bytes   | Counter | Number of bytes transmitted in multicast RDMA packets                           |                       |
| vport_speed_mbps                | Speed   | Speed of the port in Mbps                                                       |                       |
