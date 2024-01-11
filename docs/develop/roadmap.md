# roadmap

| feature            | description                                                                                                                          | Alpha release | Beta release | GA release |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------|---------------|--------------|------------|
| SpiderIppool       | ip settings                                                                                                                          | v0.2.0        | v0.4.0       | v0.6.0     |
|                    | namespace affinity                                                                                                                   | v0.4.0        | v0.6.0       |            |
|                    | application affinity                                                                                                                 | v0.4.0        | v0.6.0       |            |
|                    | multiple default ippool                                                                                                              | v0.6.0        |              |            |
|                    | multusname affinity                                                                                                                  | v0.6.0        |              |            |
|                    | nodename affinity                                                                                                                    | v0.6.0        | v0.6.0       |            |
|                    | default cluster ippool                                                                                                               | v0.2.0        | v0.4.0       | v0.6.0     |
|                    | default namespace ippool                                                                                                             | v0.4.0        | v0.5.0       |            |
|                    | default CNI ippool                                                                                                                   | v0.4.0        | v0.4.0       |            |
|                    | annotation ippool                                                                                                                    | v0.2.0        | v0.5.0       |            |
|                    | annotation route                                                                                                                     | v0.2.0        | v0.5.0       |            |
|                    | ippools for multi-interfaces without specified interface name  in annotation                                                         | v0.9.0        |              |            |
| SpiderSubnet       | automatically create ippool                                                                                                          | v0.4.0        |              |            |
|                    | automatically scaling and deletion ip according to application                                                                       | v0.4.0        |              |            |
|                    | automatically delete ippool                                                                                                          | v0.5.0        |              |            |
|                    | annotation for multiple interface                                                                                                    | v0.4.0        |              |            |
|                    | keep ippool after deleting application                                                                                               | v0.5.0        |              |            |
|                    | support deployment, statefulset, job, replicaset                                                                                     | v0.4.0        |              |            |
|                    | support operator controller                                                                                                          | v0.4.0        |              |            |
|                    | flexible ip number                                                                                                                   | v0.5.0        |              |            |
|                    | ippool inherit route and gateway attribute from its subnet                                                                           | v0.6.0        |              |            |
| reservedIP         | reservedIP                                                                                                                           | v0.4.0        | v0.6.0       |            |
| Fixed IP           | fixed ip for each pod of statefulset                                                                                                 | v0.5.0        |              |            |
|                    | fixed ip ranges for statefulset, deployment, replicaset                                                                              | v0.4.0        | v0.6.0       |            |
|                    | fixed ip for kubevirt                                                                                                                | v0.8.0        |              |            |
|                    | support calico                                                                                                                       | v0.5.0        | v0.6.0       |            |
|                    | support weave                                                                                                                        | v0.5.0        | v0.6.0       |            |
| Spidermultusconfig | support macvlan ipvlan sriov custom                                                                                                  | v0.6.0        | v0.7.0       |            |        
|                    | support ovs-cni                                                                                                                      | v0.7.0        |              |            |
| CNI version        | cni v1.0.0                                                                                                                           | v0.4.0        | v0.5.0       |            |
| ifacer             | bond interface                                                                                                                       | v0.6.0        | v0.8.0       |            |
|                    | vlan interface                                                                                                                       | v0.6.0        | v0.8.0       |            |
| SpiderCoordinator  | Sync podCIDR for calico                                                                                                              | v0.6.0        | v0.8.0       |            |
|                    | Sync podCIDR for cilium                                                                                                              | v0.6.0        | v0.8.0       |            |
|                    | sync clusterIP CIDR from serviceCIDR to support k8s 1.29                                                                             |               | v0.10.0      |            |
| Coordinator        | support underlay mode                                                                                                                | v0.6.0        | v0.7.0       |            |
|                    | support overlay mode                                                                                                                 | v0.6.0        | v0.8.0       |            |
|                    | CRD spidercoordinators for multus configuration                                                                                      | v0.6.0        | v0.8.0       |            |
|                    | detect ip conflict and gateway                                                                                                       | v0.6.0        | v0.6.0       |            |
|                    | specify the MAC of pod                                                                                                               | v0.6.0        | v0.8.0       |            |
|                    | tune the default route of pod multiple interfaces                                                                                    | v0.6.0        | v0.8.0       |            |
| Connectivity       | visit service based on kube-proxy                                                                                                    | v0.6.0        | v0.7.0       |            |
|                    | visit local node to guarantee the pod health check                                                                                   | v0.6.0        | v0.7.0       |            |
|                    | visit nodePort with spec.externalTrafficPolicy=local or spec.externalTrafficPolicy=cluster                                           | v0.6.0        |              |            |
| Observability      | eBPF: pod stats                                                                                                                      | In plan       |              |            |
| Network Policy     | ipvlan                                                                                                                               | v0.8.0        |              |            |
|                    | macvlan                                                                                                                              | In plan       |              |            |
|                    | sriov                                                                                                                                | In plan       |              |            |
| Bandwidth          | ipvlan                                                                                                                               | v0.8.0        |              |            |
|                    | macvlan                                                                                                                              | In plan       |              |            |
|                    | sriov                                                                                                                                | In plan       |              |            |
| eBPF               | implement service by cgroup eBPF                                                                                                     | v0.8.0        |              |            |
|                    | accelerate communication of pods on a same node                                                                                      | In plan       |              |            |
| Recycle IP         | recycle IP taken by deleted pod                                                                                                      | v0.4.0        | v0.6.0       |            |
|                    | recycle IP taken by deleting pod                                                                                                     | v0.4.0        | v0.6.0       |            |
|                    | recycle IP when detected IP conflict                                                                                                 | v0.10.0       |              |            |
| Dual Stack         | dual-stack                                                                                                                           | v0.2.0        | v0.4.0       |            |
| CLI                | debug and operate. check which pod an IP is taken by, check IP usage , trigger GC                                                    | In plan       |              |            |
| Multi-cluster      | a broker cluster could synchronize ippool resource within a same subnet from all member clusters, which could help avoid IP conflict | In plan       |              |            |
|                    | support submariner                                                                                                                   | v0.8.0        |              |            |
| Dual CNI           | underlay cooperate with cilium                                                                                                       | v0.7.0        |              |            |
|                    | underlay cooperate with calico                                                                                                       | v0.7.0        |              |            |
| RDMA               | support macvlan and ipvlan CNI for RoCE device                                                                                       | v0.8.0        |              |            |
|                    | support sriov CNI for RoCE device                                                                                                    | v0.8.0        |              |            |
|                    | support ipoib CNI for infiniband device                                                                                              | v0.9.0        |              |            |
|                    | support ib-sriov CNI for infiniband device                                                                                           | v0.9.0        |              |            |
| EgressGateway      | egressGateway                                                                                                                        | v0.8.0        |              |            |
