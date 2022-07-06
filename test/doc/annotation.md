# e2e case for annotation

| case id | title                                                                                                                                             | priority | smoke | status | other |
|---------|---------------------------------------------------------------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| A00001  | it fails to run a pod with different VLAN for ipv4 and ipv6 IPPool                                                                                | p3       |       | done   |       |
| A00002  | succeed to run a pod for multiple NICS with different pools                                                                                       | p2       |       | NA     |       |
| A00003  | fail to run a pod with invalid annotations                                                                                                        | p3       |       | done   |       |
| A00004  | the pod annotation has the highest priority over namespace and global default IPPool                                                              | p1       |       | NA     |       |
| A00005  | the "ippools" annotation has the higher priority over the "ippool" annotation                                                                     | p1       |       | done   |       |
| A00006  | the namespace annotation has precedence over global default IPPool                                                                                | p1       | true  | done   |       |
| A00007  | Spiderpool will successively try to allocate IP in the order of the elements in the IPPool array until the first allocation succeeds or all fail  | p1       | true  | NA     |       |
