# Metric

Spiderpool can be configured to serve [Opentelemetry](https://opentelemetry.io/) metrics.
And spiderpool metrics provide the insight of Spiderpool Agent and Spiderpool Controller.

## spiderpool controller

The metrics of spiderpool controller is set by the following pod environment:

| environment                     | description                | default |
|---------------------------------|----------------------------| ------- |
| SPIDERPOOL_ENABLED_METRIC       | enable metrics             | false   |
| SPIDERPOOL_ENABLED_DEBUG_METRIC | enable debug level metrics | false   |
| SPIDERPOOL_METRIC_HTTP_PORT     | metrics port               | 5721    |

## spiderpool agent

The metrics of spiderpool agent is set by the following pod environment:

| environment                     | description                | default |
|---------------------------------|----------------------------|---------|
| SPIDERPOOL_ENABLED_METRIC       | enable metrics             | false   |
| SPIDERPOOL_ENABLED_DEBUG_METRIC | enable debug level metrics | false   |
| SPIDERPOOL_METRIC_HTTP_PORT     | metrics port               | 5711    |

## Get Started

### Enable Metric support

Check the environment variable `SPIDERPOOL_ENABLED_METRIC` of the daemonset `spiderpool-agent` for whether it is already set to `true` or not.

Check the environment variable `SPIDERPOOL_ENABLED_METRIC` of deployment `spiderpool-controller` for whether it is already set to `true` or not.

```shell
kubectl -n kube-system get daemonset spiderpool-agent -o yaml
------
kubectl -n kube-system get deployment spiderpool-controller -o yaml
```

You can set one or both of them to `true`.
For example, let's enable spiderpool agent metrics by running `helm upgrade --set spiderpoolAgent.prometheus.enabled=true`.

## Metric reference

### Spiderpool Agent

Spiderpool agent exports some metrics related with IPAM allocation and release. Currently, those include:


| Name                                                      | description                                                                                                                       |
|-----------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| spiderpool_ipam_allocation_counts                         | Number of IPAM allocation requests that Spiderpool Agent received , prometheus type: counter                                      |
| spiderpool_ipam_allocation_failure_counts                 | Number of Spiderpool Agent IPAM allocation failures, prometheus type: counter                                                     |
| spiderpool_ipam_allocation_update_ippool_conflict_counts  | Number of Spiderpool Agent IPAM allocation update IPPool conflicts, prometheus type: counter                                      |
| spiderpool_ipam_allocation_err_internal_counts            | Number of Spiderpool Agent IPAM allocation internal errors, prometheus type: counter                                              |
| spiderpool_ipam_allocation_err_no_available_pool_counts   | Number of Spiderpool Agent IPAM allocation no available IPPool errors, prometheus type: counter                                   |
| spiderpool_ipam_allocation_err_retries_exhausted_counts   | Number of Spiderpool Agent IPAM allocation retries exhausted errors, prometheus type: counter                                     |
| spiderpool_ipam_allocation_err_ip_used_out_counts         | Number of Spiderpool Agent IPAM allocation IP addresses used out errors, prometheus type: counter                                 |
| spiderpool_ipam_allocation_average_duration_seconds       | The average duration of all Spiderpool Agent allocation processes, prometheus type: gauge                                         |
| spiderpool_ipam_allocation_max_duration_seconds           | The maximum duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                                 |
| spiderpool_ipam_allocation_min_duration_seconds           | The minimum duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                                 |
| spiderpool_ipam_allocation_latest_duration_seconds        | The latest duration of Spiderpool Agent allocation process (per-process), prometheus type: gauge                                  |
| spiderpool_ipam_allocation_duration_seconds               | Histogram of IPAM allocation duration in seconds, prometheus type: histogram                                                      |
| spiderpool_ipam_allocation_average_limit_duration_seconds | The average duration of all Spiderpool Agent allocation queuing, prometheus type: gauge                                           |
| spiderpool_ipam_allocation_max_limit_duration_seconds     | The maximum duration of Spiderpool Agent allocation queuing, prometheus type: gauge                                               |
| spiderpool_ipam_allocation_min_limit_duration_seconds     | The minimum duration of Spiderpool Agent allocation queuing, prometheus type: gauge                                               |
| spiderpool_ipam_allocation_latest_limit_duration_seconds  | The latest duration of Spiderpool Agent allocation queuing, prometheus type: gauge                                                |
| spiderpool_ipam_allocation_limit_duration_seconds         | Histogram of IPAM allocation queuing duration in seconds, prometheus type: histogram                                              |
| spiderpool_ipam_release_counts                            | Count of the number of Spiderpool Agent received the IPAM release requests, prometheus type: counter                              |
| spiderpool_ipam_release_failure_counts                    | Number of Spiderpool Agent IPAM release failure, prometheus type: counter                                                         |
| spiderpool_ipam_release_update_ippool_conflict_counts     | Number of Spiderpool Agent IPAM release update IPPool conflicts, prometheus type: counter                                         |
| spiderpool_ipam_release_err_internal_counts               | Number of Spiderpool Agent IPAM releasing internal error, prometheus type: counter                                                |
| spiderpool_ipam_release_err_retries_exhausted_counts      | Number of Spiderpool Agent IPAM releasing retries exhausted error, prometheus type: counter                                       |
| spiderpool_ipam_release_average_duration_seconds          | The average duration of all Spiderpool Agent release processes, prometheus type: gauge                                            |
| spiderpool_ipam_release_max_duration_seconds              | The maximum duration of Spiderpool Agent release process (per-process), prometheus type: gauge                                    |
| spiderpool_ipam_release_min_duration_seconds              | The minimum duration of Spiderpool Agent release process (per-process), prometheus type: gauge                                    |
| spiderpool_ipam_release_latest_duration_seconds           | The latest duration of Spiderpool Agent release process (per-process), prometheus type: gauge                                     |
| spiderpool_ipam_release_duration_seconds                  | Histogram of IPAM release duration in seconds, prometheus type: histogram                                                         |
| spiderpool_ipam_release_average_limit_duration_seconds    | The average duration of all Spiderpool Agent release queuing, prometheus type: gauge                                              |
| spiderpool_ipam_release_max_limit_duration_seconds        | The maximum duration of Spiderpool Agent release queuing, prometheus type: gauge                                                  |
| spiderpool_ipam_release_min_limit_duration_seconds        | The minimum duration of Spiderpool Agent release queuing, prometheus type: gauge                                                  |
| spiderpool_ipam_release_latest_limit_duration_seconds     | The latest duration of Spiderpool Agent release queuing, prometheus type: gauge                                                   |
| spiderpool_ipam_release_limit_duration_seconds            | Histogram of IPAM release queuing duration in seconds, prometheus type: histogram                                                 |
| spiderpool_debug_auto_pool_waited_for_available_counts    | Number of Spiderpool Agent IPAM allocation wait for auto-created IPPool available, prometheus type: counter. (debug level metric) |

### Spiderpool Controller

Spiderpool controller exports some metrics related with SpiderIPPool IP garbage collection. Currently, those include:

| Name                                                   | description                                                                                                        |
|--------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------|
| spiderpool_ip_gc_counts                                | Number of Spiderpool Controller IP garbage collection, prometheus type: counter.                                   |
| spiderpool_ip_gc_failure_counts                        | Number of Spiderpool Controller IP garbage collection failures, prometheus type: counter.                          |
| spiderpool_total_ippool_counts                         | Number of Spiderpool IPPools, prometheus type: gauge.                                                              |
| spiderpool_debug_ippool_total_ip_counts                | Number of Spiderpool IPPool corresponding total IPs (per-IPPool), prometheus type: gauge. (debug level metric)     |
| spiderpool_debug_ippool_available_ip_counts            | Number of Spiderpool IPPool corresponding availbale IPs (per-IPPool), prometheus type: gauge. (debug level metric) |
| spiderpool_total_subnet_counts                         | Number of Spiderpool Subnets, prometheus type: gauge.                                                              |
| spiderpool_debug_subnet_ippool_counts                  | Number of Spiderpool Subnet corresponding IPPools (per-Subnet), prometheus type: gauge. (debug level metric)       |
| spiderpool_debug_subnet_total_ip_counts                | Number of Spiderpool Subnet corresponding total IPs (per-Subnet), prometheus type: gauge. (debug level metric)     |
| spiderpool_debug_subnet_available_ip_counts            | Number of Spiderpool Subnet corresponding availbale IPs (per-Subnet), prometheus type: gauge. (debug level metric) |
| spiderpool_debug_auto_pool_waited_for_available_counts | Number of waiting for auto-created IPPool available, prometheus type: couter. (debug level metric)                 |


### RDMA exporter

Spiderpool also provides RDMA exporter to export RDMA metrics. The RDMA metrics include:

| Metric Name                          | Type    | Description                                                                     | Remarks               |
|--------------------------------------|---------|---------------------------------------------------------------------------------|-----------------------|
| rdma_rx_write_requests               | Counter | Number of received write requests                                               |                       |
| rdma_rx_read_requests                | Counter | Number of received read requests                                                |                       |
| rdma_rx_atomic_requests              | Counter | Number of received atomic requests                                              |                       |
| rdma_rx_dct_connect                  | Counter | Number of received DCT connection requests                                      |                       |
| rdma_out_of_buffer                   | Counter | Number of buffer insufficiency errors                                           |                       |
| rdma_out_of_sequence                 | Counter | Number of out-of-sequence packets received                                      |                       |
| rdma_duplicate_request               | Counter | Number of duplicate requests                                                    |                       |
| rdma_rnr_nak_retry_err               | Counter | Count of RNR NAK packets not exceeding QP retry limit                           |                       |
| rdma_packet_seq_err                  | Counter | Number of packet sequence errors                                                |                       |
| rdma_implied_nak_seq_err             | Counter | Number of implied NAK sequence errors                                           |                       |
| rdma_local_ack_timeout_err           | Counter | Number of times the sender's QP ack timer expired                               | RC, XRC, DCT QPs only |
| rdma_resp_local_length_error         | Counter | Number of times a respondent detected a local length error                      |                       |
| rdma_resp_cqe_error                  | Counter | Number of response CQE errors                                                   |                       |
| rdma_req_cqe_error                   | Counter | Number of times a requester detected CQE completion with errors                 |                       |
| rdma_req_remote_invalid_request      | Counter | Number of remote invalid request errors detected by requester                   |                       |
| rdma_req_remote_access_errors        | Counter | Number of requested remote access errors                                        |                       |
| rdma_resp_remote_access_errors       | Counter | Number of response remote access errors                                         |                       |
| rdma_resp_cqe_flush_error            | Counter | Number of response CQE flush errors                                             |                       |
| rdma_req_cqe_flush_error             | Counter | Number of request CQE flush errors                                              |                       |
| rdma_roce_adp_retrans                | Counter | Number of RoCE adaptive retransmissions                                         |                       |
| rdma_roce_adp_retrans_to             | Counter | Number of RoCE adaptive retransmission timeouts                                 |                       |
| rdma_roce_slow_restart               | Counter | Number of RoCE slow restarts                                                    |                       |
| rdma_roce_slow_restart_cnps          | Counter | Number of CNP packets generated during RoCE slow restart                        |                       |
| rdma_roce_slow_restart_trans         | Counter | Number of times state transitioned to slow restart                              |                       |
| rdma_rp_cnp_ignored                  | Counter | Number of CNP packets received and ignored by Reaction Point HCA                |                       |
| rdma_rp_cnp_handled                  | Counter | Number of CNP packets handled by Reaction Point HCA to reduce transmission rate |                       |
| rdma_np_ecn_marked_roce_packets      | Counter | Number of ECN-marked RoCE packets indicating path congestion                    |                       |
| rdma_np_cnp_sent                     | Counter | Number of CNP packets sent when congestion is experienced in RoCEv2 IP header   |                       |
| rdma_rx_icrc_encapsulated            | Counter | Number of RoCE packets with ICRC errors                                         |                       |
| rdma_rx_vport_rdma_unicast_packets   | Counter | Number of received unicast RDMA packets                                         |                       |
| rdma_tx_vport_rdma_unicast_packets   | Counter | Number of transmitted unicast RDMA packets                                      |                       |
| rdma_rx_vport_rdma_multicast_packets | Counter | Number of received multicast RDMA packets                                       |                       |
| rdma_tx_vport_rdma_multicast_packets | Counter | Number of transmitted multicast RDMA packets                                    |                       |
| rdma_rx_vport_rdma_unicast_bytes     | Counter | Number of bytes received in unicast RDMA packets                                |                       |
| rdma_tx_vport_rdma_unicast_bytes     | Counter | Number of bytes transmitted in unicast RDMA packets                             |                       |
| rdma_rx_vport_rdma_multicast_bytes   | Counter | Number of bytes received in multicast RDMA packets                              |                       |
| rdma_tx_vport_rdma_multicast_bytes   | Counter | Number of bytes transmitted in multicast RDMA packets                           |                       |
| rdma_vport_speed_mbps                | Speed   | Speed of the port in Mbps                                                       |                       |

