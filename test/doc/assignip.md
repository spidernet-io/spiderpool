# e2e case for assign ip

| case id   | title                                                                                      | priority | smoke | status |    other    |
|---------|----------------------------------------------------------------------------------------------|----------|-------|--------|-------------|
| E00001  | assign IP to a pod for ipv4, ipv6 and dual-stack case                                        | p1       | true  | done   |             |
| E00002  | assign IP to deployment/pod for ipv4, ipv6 and dual-stack case                               | p1       | true  | done   |             |
| E00003  | assign IP to statefulSet/pod for ipv4, ipv6 and dual-stack case                              | p1       | true  | done   |             |
| E00004  | assign IP to daemonSet/pod for ipv4, ipv6 and dual-stack case                                | p1       | true  | done   |             |
| E00005  | assign IP to job/pod for ipv4, ipv6 and dual-stack case                                      | p1       | true  | done   |             |
| E00006  | assign IP to replicaset/pod for ipv4, ipv6 and dual-stack case                               | p1       | true  | done   |             |
| E00007  | succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case                       | p2       |       | done   |             |
| E00008  | failed to run a pod when IP resource of an IPPool is exhausted                               | p3       |       | done   |             |
