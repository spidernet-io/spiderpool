# Network I/O Performance

**English** | [**简体中文**](./io-performance-zh_CN.md)

*[Spiderpool](https://github.com/spidernet-io/spiderpool) can be used with Macvlan, SR-IOV, and IPvlan to implement a complete network solution. This article will compare it with the mainstream network CNI plug-ins on the market ( Such as [cilium](https://github.com/cilium/cilium), [calico](https://github.com/projectcalico/calico)) Network `Latency` and `Throughput` in various scenarios*

## ENV

This test contains performance benchmark data for various scenarios. All tests were performed between containers running on two different bare metal nodes with 10 Gbit/s network interfaces.

- Kubernetes: `v1.28.2`
- container runtime: `containerd 1.6.24`
- OS: `ubuntu 23.04`
- kernel: `6.2.0-35-generic`
- NIC: `Mellanox Technologies MT27800 Family [ConnectX-5]`

| Node     | Role                  | CPU | Memory |
| -------- | --------------------- | --- | ------ |
| master1  | control-plane, worker | 56C | 125Gi  |
| worker1  | worker                | 56C | 125Gi  |

## Test object

This test uses [macvlan](https://www.cni.dev/plugins/current/main/macvlan/) with Spiderpool as the test solution, and selected [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium) For comparison, two common network solutions are as follows. The following is the relevant version and other information:

|             Test object            |            illustrate                                                    |
| ---------------------------------- | ------------------------------------------------------------------------ |
| Spiderpool based macvlan datapath  | Spiderpool version v0.8.0                                                |
| Calico                             | Calico version v3.26.1, based on iptables datapath and no tunnels        |
| Cilium                             | Cilium version v1.14.3, based on full eBPF acceleration and no tunneling |

## sockperf network latency test

Sockperf is a network benchmarking tool that can be used to measure network latency. It allows you to evaluate the performance of your network by testing the latency between two endpoints. We can use it to separately test Pod's cross-node access to Pod and Service. When testing access to Service's cluster IP, there are two scenarios: `kube-proxy` or `cilium + kube-proxy replacement`.

- Cross-node Pod latency testing for Pod IP purposes.

  Use `sockperf pp --tcp -i <Pod IP> -p 12345 -t 30` to test the latency of cross-node Pod access to the Pod IP. The data is as follows.

  | Test object                                             |       latency        |
  | ------------------------------------------------------- | -------------------- |
  | Calico based on iptables datapath and tunnelless        |       51.3 usec      |
  | Cilium based on full eBPF acceleration and no tunneling |       29.1 usec      |
  | Spiderpool Pod on the same subnet based on macvlan      |       24.3 usec      |
  | Spiderpool Pod across subnets based on macvlan          |       26.2 usec      |
  | node to node                                            |       32.2 usec      |

- Cross-node Pod latency test for cluster IP purpose.

  Use `sockperf pp --tcp -i <Cluster IP> -p 12345 -t 30` to test the latency of cross-node Pod access to the cluster IP. The data is as follows.

  | Test object                                                                   |     latency    |
  | ----------------------------------------------------------------------------- | -------------- |
  | Calico based on iptables datapath and tunnelless                              |   51.9 usec    |
  | Cilium based on full eBPF acceleration and no tunneling                       |   30.2 usec    |
  | Spiderpool Pod based on macvlan on the same subnet and kube-proxy             |   36.8 usec    |
  | Spiderpool Pod based on macvlan on the same subnet and fully eBPF accelerated |   27.7 usec    |
  | node to node                                                                  |   32.2 usec    |

![performance](../images/performance-sockperf.png)

## netperf performance test

netperf is a widely used network performance testing tool that allows you to measure various aspects of network performance, such as throughput. We can use netperf to test Pod's cross-node access to Pod and Service respectively. When testing access to Service's cluster IP, there are two scenarios: `kube-proxy` or `cilium + kube-proxy replacement`.

- Netperf testing of cross-node Pods for Pod IP purposes.
  
  Use `netperf -H <Pod IP> -l 10 -c -t TCP_RR -- -r100,100` to test the throughput of cross-node Pod access to Pod IP. The data is as follows.

  | Test object                                              |  Throughput (rps) |
  | -------------------------------------------------------- | ----------------- |
  | Calico based on iptables datapath and tunnelless         |     9985.7        |
  | Cilium based on full eBPF acceleration and no tunneling  |     17571.3       |
  | Spiderpool Pod on the same subnet based on macvlan       |     19793.9       |
  | Spiderpool Pod across subnets based on macvlan           |     19215.2       |
  | node to node                                             |     47560.5       |

- Netperf testing across node Pods for cluster IP purposes.

  Use `netperf -H <cluster IP> -l 10 -c -t TCP_RR -- -r100,100` to test the throughput of cross-node Pods accessing the cluster IP. The data is as follows.

  | Test object                                                                    | Throughput (rps) |
  | ------------------------------------------------------------------------------ | ---------------- |
  | Calico based on iptables datapath and tunnelless                               |     9782.2       |
  | Cilium based on full eBPF acceleration and no tunneling                        |     17236.5      |
  | Spiderpool Pod based on macvlan on the same subnet and kube-proxy              |     16002.3      |
  | Spiderpool Pod based on macvlan on the same subnet and fully eBPF accelerated  |     18992.9      |
  | node to node                                                                   |     47560.5      |

## iperf network performance test

iperf is a popular network performance testing tool that allows you to measure network bandwidth between two endpoints. It is widely used to evaluate the bandwidth and performance of network connections. In this chapter, we use it to test Pod's cross-node access to Pod and Service. When testing access to Service's cluster IP, there are two scenarios: `kube-proxy` or `cilium + kube-proxy replacement`.

- iperf testing of cross-node Pods for Pod IP purposes.
  
  Use `iperf3 -c <Pod IP> -d -P 1` to test the performance of cross-node Pod access to Pod IP. Use the -P parameter to specify threads 1, 2, and 4 respectively. The data is as follows.

  | Test object                                              |   Number of threads 1 |  Number of threads 2  |  Number of threads 4 |
  | -------------------------------------------------------- | -------------------- | -------------------- | -------------------- |
  | Calico based on iptables datapath and tunnelless         |   3.26 Gbits/sec  |    4.56 Gbits/sec    |   8.05 Gbits/sec     |
  | Cilium based on full eBPF acceleration and no tunneling  |   9.35 Gbits/sec  |    9.36 Gbits/sec    |   9.39 Gbits/sec     |
  | Spiderpool Pod on the same subnet based on macvlan       |   9.36 Gbits/sec  |    9.37 Gbits/sec    |   9.38 Gbits/sec     |
  | Spiderpool Pod across subnets based on macvlan           |   9.36 Gbits/sec  |    9.37 Gbits/sec    |   9.38 Gbits/sec     |
  | node to node                                             |   9.41 Gbits/sec  |    9.40 Gbits/sec    |   9.42 Gbits/sec     |

- iperf testing of cross-node Pods for cluster IP purposes.

  Use `iperf3 -c <cluster IP> -d -P 1` to test the performance of cross-node Pod access to cluster IP. Use the -P parameter to specify threads 1, 2, and 4 respectively. The data is as follows.

  | Test object                                                                   |  Number of threads 1 |  Number of threads 2 |  Number of threads 4 |
  | ----------------------------------------------------------------------------- | ----------------- | ----------------- | --------------- |
  | Calico based on iptables datapath and tunnelless                              |   3.06 Gbits/sec  |  4.63 Gbits/sec  |  8.02 Gbits/sec  |
  | Cilium based on full eBPF acceleration and no tunneling                       |  9.35 Gbits/sec   |  9.35 Gbits/sec  |  9.38 Gbits/sec  |
  | Spiderpool Pod based on macvlan on the same subnet and kube-proxy             |  3.42 Gbits/sec   |  6.75 Gbits/sec  |  9.24 Gbits/sec  |
  | Spiderpool Pod based on macvlan on the same subnet and fully eBPF accelerated |  9.36 Gbits/sec   |  9.38 Gbits/sec  |  9.39 Gbits/sec  |
  | node to node                                                                  |   9.41 Gbits/sec  |  9.40 Gbits/sec  |  9.42 Gbits/sec  |

## redis-benchmark performance test

redis-benchmark is designed to measure the performance and throughput of a Redis server by simulating multiple clients and executing various Redis commands. We used redis-benchmark to test Pod's cross-node access to the Pod and Service where the Redis service is deployed. When testing access to Service's cluster IP, there are two scenarios: `kube-proxy` or `cilium + kube-proxy replacement`.

- Cross-node Pod redis-benchmark testing based on Pod IP.

  Use `redis-benchmark -h <Pod IP> -p 6379 -d 1000 -t get,set` to test the performance of cross-node Pod access to Pod IP. The data is as follows.

  | Test object                                               |         get          |        set         |
  | --------------------------------------------------------- | -------------------- | ------------------ |
  | Calico based on iptables datapath and tunnelless          |      45682.96 rps    |     46992.48 rps   |
  | Cilium based on full eBPF acceleration and no tunneling   |      59737.16 rps    |     59988.00 rps   |
  | Spiderpool Pod on the same subnet based on macvlan        |      66357.00 rps    |     66800.27 rps   |
  | Spiderpool Pod across subnets based on macvlan            |      67444.45 rps    |     67783.67 rps   |

- Cross-node Pod redis-benchmark testing for cluster IP purposes.

  Use `redis-benchmark -h <cluster IP> -p 6379 -d 1000 -t get,set` to test the performance of cross-node Pod access to cluster IP. The data is as follows.

  | Test object                                                                    |        get            |        set           |
  | ------------------------------------------------------------------------------ | --------------------- | -------------------- |
  | Calico based on iptables datapath and tunnelless                               |       46082.95 rps    |      46728.97 rps    |
  | Cilium based on full eBPF acceleration and no tunneling                        |       60496.07 rps    |      58927.52 rps    |
  | Spiderpool Pod based on macvlan on the same subnet and kube-proxy              |       45578.85 rps    |      46274.87 rps    |
  | Spiderpool Pod based on macvlan on the same subnet and fully eBPF accelerated  |       63211.12 rps    |      64061.50 rps    |

![performance](../images/performance-redis.png)

## Same node eBPF acceleration test

Spiderpool can achieve same-node communication acceleration with the help of the [istio-tcpip-bypass](https://github.com/intel/istio-tcpip-bypass) project. Run the service on one node of the cluster and not on the other node. Conduct a performance test through Sockperf between Pods on the same node. The data is as follows.

  | Test object                                 |        latency       |  
  | ------------------------------------------- | -------------------- |
  |  Node enables eBPF acceleration             |      7.643 usec      |
  |  Node is not enabled for eBPF acceleration  |      17.335 usec     |

## Summary

When Spiderpool is used as an underlay network solution, its IO performance is ahead of Calico and Cilium in most scenarios.
