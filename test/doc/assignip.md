# e2e case for assign ip

| case id   | title                                                                                      | priority | smoke | status | other |
|---------|--------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| E00001  | assign IP to a pod for ipv4, ipv6 and dual-stack case                                      | p1       | true  | done   |       |
| E00002  | assign IP to deployment/pod for ipv4, ipv6 and dual-stack case                             | p1       | true  | done   |       |
| E00003  | assign IP to statefulSet/pod for ipv4, ipv6 and dual-stack case                            | p1       | true  | done   |       |
| E00004  | assign IP to daemonset/pod for ipv4, ipv6 and dual-stack case                              | p1       | true  | done   |       |
| E00005  | assign IP to job/pod for ipv4, ipv6 and dual-stack case                                    | p1       | true  | NA     |       |
| E00006  | assign IP to replicaset/pod for ipv4, ipv6 and dual-stack case                             | p1       | true  | done   |       |
| E00007  | succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case                     | p2       |       | done   |       |
| E00008  | succeed to run deployment/pod who is bound to an ippool set with matched nodeSelector      | p2       |       | NA     |       |
| E00009  | fail to run deployment/pod who is bound to an ippool set with no-matched nodeSelector      | p3       |       | NA     |       |
| E00010  | succeed to run deployment/pod who is bound to an ippool set with matched namesapceSelector | p2       |       | NA     |       |
| E00011  | fail to run deployment/pod who is bound to an ippool set with no-matched namesapceSelector | p3       |       | NA     |       |
| E00012  | succeed to run deployment/pod who is bound to an ippool set with matched podSelector       | p2       |       | NA     |       |
| E00013  | fail to run deployment/pod who is bound to an ippool set with no-matched podSelector       | p3       |       | NA     |       |
| E00014  | succeed to create and delete ippool in batches                                             | p2       |       | NA     |       |
| E00015  | fail to run a pod when IP resource of an ippool is exhausted                                | p3       |       | NA     |       |
