# e2e case for annotation

| case id | title                                                                                | priority | smoke | status | other |
|---------|--------------------------------------------------------------------------------------|----------|-------|--------|-------|
| A00001  | it fails to run a pod with different VLAN for ipv4 and ipv6 ippool                   | p2       |       | NA     |       |
| A00002  | succeed to run a pod for multiple NICS with different pools                          | p2       |       | NA     |       |
| A00003  | fail to run a pod with invalid annotations                                           | p2       |       | NA     |       |
| A00004  | the pod annotation has the highest priority over namespace and global default ippool | p2       |       | NA     |       |
| A00005  | the "ippools" annotation has the higher priority over the "ippool" annotation        | p2       |       | NA     |       |
