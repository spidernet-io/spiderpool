# roadmap

| feature                  | description                                                                                                                          | status  | release |
|--------------------------|--------------------------------------------------------------------------------------------------------------------------------------|---------|---------|
| ippool                   | ip settings                                                                                                                          | GA      | v0.2.0  |
|                          | namesapce affinity                                                                                                                   | Beta    | v0.4.0  |
|                          | application affinity                                                                                                                 | Beta    | v0.4.0  |
|                          | multiple default ippool                                                                                                              | Beta    | v0.6.0  |
|                          | multusname                                                                                                                           | Alpha   | v0.6.0  |
|                          | nodename                                                                                                                             | Beta    | v0.6.0  |
|                          | default cluster ippool                                                                                                               | GA      | v0.2.0  |
|                          | default namespace ippool                                                                                                             | Beta    | v0.4.0  |
|                          | default CNI ippool                                                                                                                   | GA      | v0.4.0  |
|                          | annotation ippool                                                                                                                    | Beta    | v0.2.0  |
|                          | annotation route                                                                                                                     | Beta    | v0.2.0  |
| subnet                   | automatically create ippool                                                                                                          | Beta    | v0.4.0  |
|                          | automatically scaling and deletion ip according to application                                                                       | Beta    | v0.4.0  |
|                          | automatically delete ippool                                                                                                          | Beta    | v0.5.0  |
|                          | support annotation for multiple interface                                                                                            | Beta    | v0.4.0  |
|                          | keep ippool after deleting application                                                                                               | Beta    | v0.5.0  |
|                          | support deployment, statefulset, job, replicaset                                                                                     | Beta    | v0.4.0  |
|                          | support operator controller                                                                                                          | Alpha   | v0.4.0  |
|                          | flexible ip number                                                                                                                   | Beta    | v0.5.0  |
|                          | ippool inherit route and gateway attribute from its subnet                                                                           | Beta    | v0.6.0  |
| reservedIP               | reservedIP                                                                                                                           | Beta    | v0.4.0  |
| fixed ip                 | fixed ip for each pod of statefulset                                                                                                 | Beta    | v0.5.0  |
|                          | fixed ip ranges for statefulset, deployment, replicaset                                                                              | Beta    | v0.4.0  |
|                          | fixed ip for kubevirt                                                                                                                | In plan |         |
|                          | support calico                                                                                                                       | Beta    | v0.5.0  |
|                          | support weave                                                                                                                        | Beta    | v0.5.0  |
| spidermultusconfig       | support macvlan ipvlan sriov custom                                                                                                  | Beta    | v0.6.0  |         
|                          | support ovs-cni                                                                                                                      | Alpha   | v0.7.0  | 
| ipam plugin              | cni v1.0.0                                                                                                                           | Beta    | v0.4.0  |
| ifacer plugin            | bond interface                                                                                                                       | Alpha   | v0.6.0  |
|                          | vlan interface                                                                                                                       | Alpha   | v0.6.0  |
| coordinator plugin       | support underlay mode                                                                                                                | Beta    | v0.6.0  |
|                          | support underlay mode                                                                                                                | Alpha   | v0.6.0  |
|                          | CRD spidercoordinators for multus configuration                                                                                      | Beta    | v0.6.0  |
|                          | detect ip conflict and gateway                                                                                                       | Beta    | v0.6.0  |
|                          | specify the MAC of pod                                                                                                               | Beta    | v0.6.0  |
|                          | tune the default route of pod multiple interfaces                                                                                    | Alpha   | v0.6.0  |
| ovs/macvlan/sriov/ipvlan | visit clusterIP                                                                                                                      | Beta    | v0.6.0  |
|                          | visit local node to guarantee the pod health check                                                                                   | Beta    | v0.6.0  |
|                          | visit nodePort with spec.externalTrafficPolicy=local or spec.externalTrafficPolicy=cluster                                           | Beta    | v0.6.0  |
|                          | network policy                                                                                                                       | In plan |         |
| recycle IP               | recycle IP taken by deleted pod                                                                                                      | Beta    | v0.4.0  |
|                          | recycle IP taken by deleting pod                                                                                                     | Beta    | v0.4.0  |
| dual-stack               | dual-stack                                                                                                                           | Beta    | v0.2.0  |
| CLI                      | debug and operate. check which pod an IP is taken by, check IP usage , trigger GC                                                    | In plan |         |
| multi-cluster            | a broker cluster could synchronize ippool resource within a same subnet from all member clusters, which could help avoid IP conflict | In plan |         |
| cilium                   | cooperate with cilium                                                                                                                | In plan |         |
| RDMA                     | RDMA                                                                                                                                 | In plan |         |
| egressGateway            | egressGateway                                                                                                                        | In plan |         |
