# E2E Cases for IP Assignment

| Case ID | Title                                                                                                 | Priority | Smoke | Status |    Other    |
|---------|-------------------------------------------------------------------------------------------------------|----------|-------|--------|-------------|
| E00001  | Assign IP to a pod for ipv4, ipv6 and dual-stack case                                                 | p1       | true  | done   |             |
| E00002  | Assign IP to deployment/pod for ipv4, ipv6 and dual-stack case                                        | p1       | true  | done   |             |
| E00003  | Assign IP to statefulSet/pod for ipv4, ipv6 and dual-stack case                                       | p1       | true  | done   |             |
| E00004  | Assign IP to daemonSet/pod for ipv4, ipv6 and dual-stack case                                         | p1       | true  | done   |             |
| E00005  | Assign IP to job/pod for ipv4, ipv6 and dual-stack case                                               | p1       | true  | done   |             |
| E00006  | Assign IP to replicaset/pod for ipv4, ipv6 and dual-stack case                                        | p1       | true  | done   |             |
| E00007  | Successfully run a pod with long yaml for ipv4, ipv6 and dual-stack case                              | p2       |       | done   |             |
| E00008  | Failed to run a pod when IP resource of an IPPool is exhausted                                        | p3       |       | done   |             |
| E00009  | The cluster is dual stack, but the spiderpool can allocates ipv4 or ipv6 only with IPPools annotation | p2       |       | done   |             |
| E00010  | The cluster is dual stack, but the spiderpool can allocates ipv4 or ipv6 only with Subnet annotation  | p2       |       | done   |             |
