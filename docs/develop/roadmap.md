# roadmap

| feature                  | description                                                                                                                          | Alpha release | Beta release | GA release |
|--------------------------|--------------------------------------------------------------------------------------------------------------------------------------|---------------|--------|------------|
| ippool                   | ip settings                                                                                                                          | v0.2.0        | v0.4.0 | v0.6.0     |
|                          | namesapce affinity                                                                                                                   | v0.4.0        | v0.6.0 |            |
|                          | application affinity                                                                                                                 | v0.4.0        | v0.6.0 |            |
|                          | multiple default ippool                                                                                                              | v0.6.0        |        |            |
|                          | multusname                                                                                                                           | v0.6.0        |        |            |
|                          | nodename                                                                                                                             | v0.6.0        | v0.6.0 |
|                          | default cluster ippool                                                                                                               | v0.2.0        | v0.4.0 | v0.6.0     |
|                          | default namespace ippool                                                                                                             | v0.4.0        | v0.5.0 |            |
|                          | default CNI ippool                                                                                                                   | v0.4.0        | v0.4.0 |            |
|                          | annotation ippool                                                                                                                    | v0.2.0        | v0.5.0 |            |
|                          | annotation route                                                                                                                     | v0.2.0        | v0.5.0 |            |
| subnet                   | automatically create ippool                                                                                                          | v0.4.0        |        |            |
|                          | automatically scaling and deletion ip according to application                                                                       | v0.4.0        |        |            |
|                          | automatically delete ippool                                                                                                          | v0.5.0        |        |            |
|                          | annotation for multiple interface                                                                                                    | v0.4.0        |        |            |
|                          | keep ippool after deleting application                                                                                               | v0.5.0        |        |            |
|                          | support deployment, statefulset, job, replicaset                                                                                     | v0.4.0        |        |            |
|                          | support operator controller                                                                                                          | v0.4.0        |        |            |
|                          | flexible ip number                                                                                                                   | v0.5.0        |        |            |
|                          | ippool inherit route and gateway attribute from its subnet                                                                           | v0.6.0        |        |            |
| reservedIP               | reservedIP                                                                                                                           | v0.4.0        | v0.6.0 |            |
| fixed ip                 | fixed ip for each pod of statefulset                                                                                                 | v0.5.0        |        |            |
|                          | fixed ip ranges for statefulset, deployment, replicaset                                                                              | v0.4.0        | v0.6.0 |            |
|                          | fixed ip for kubevirt                                                                                                                | In plan       |        |            |
|                          | support calico                                                                                                                       | v0.5.0        | v0.6.0 |            |
|                          | support weave                                                                                                                        | v0.5.0        | v0.6.0 |            |
| spidermultusconfig       | support macvlan ipvlan sriov custom                                                                                                  | v0.6.0        | v0.7.0 |            |        
|                          | support ovs-cni                                                                                                                      | v0.7.0        |        |            |
| ipam plugin              | cni v1.0.0                                                                                                                           | v0.4.0        | v0.5.0 |            |
| ifacer plugin            | bond interface                                                                                                                       | v0.6.0        |        |            |
|                          | vlan interface                                                                                                                       | v0.6.0        |        |            |
| coordinator plugin       | support underlay mode                                                                                                                | v0.6.0        | v0.7.0 |            |
|                          | support overlay mode                                                                                                                 | v0.6.0        |        |            |
|                          | CRD spidercoordinators for multus configuration                                                                                      | v0.6.0        |        |            |
|                          | detect ip conflict and gateway                                                                                                       | v0.6.0        | v0.6.0 |            |
|                          | specify the MAC of pod                                                                                                               | v0.6.0        |        |            |
|                          | tune the default route of pod multiple interfaces                                                                                    | v0.6.0        |        |            |
| ovs/macvlan/sriov/ipvlan | visit clusterIP                                                                                                                      | v0.6.0        | v0.7.0 |            |
|                          | visit local node to guarantee the pod health check                                                                                   | v0.6.0        | v0.7.0 |            |
|                          | visit nodePort with spec.externalTrafficPolicy=local or spec.externalTrafficPolicy=cluster                                           | v0.6.0        |        |            |
|                          | network policy                                                                                                                       | In plan       |        |            |
| recycle IP               | recycle IP taken by deleted pod                                                                                                      | v0.4.0        | v0.6.0 |            |
|                          | recycle IP taken by deleting pod                                                                                                     | v0.4.0        | v0.6.0 |            |
| dual-stack               | dual-stack                                                                                                                           | v0.2.0        | v0.4.0 |            |
| CLI                      | debug and operate. check which pod an IP is taken by, check IP usage , trigger GC                                                    | In plan       |        |            |
| multi-cluster            | a broker cluster could synchronize ippool resource within a same subnet from all member clusters, which could help avoid IP conflict | In plan       |        |            |
|                          | support submariner                                                                                                                   | In plan       |        |            |
| cilium                   | cooperate with cilium                                                                                                                | In plan       |        |            |
| RDMA                     | support macvlan and ipvlan CNI for roce device                                                                                       | v0.8.0        |        |            |
|                          | support sriov CNI for roce device                                                                                                    | v0.8.0        |        |            |
|                          | support ipoib CNI for infiniband device                                                                                              | In plan       |        |            |
|                          | support ib-sriov CNI for infiniband device                                                                                           | In plan       |        |            |
| egressGateway            | egressGateway                                                                                                                        | In plan       |        |            |
