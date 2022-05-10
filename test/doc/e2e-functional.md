# e2e case for functional test
### https://dwiki.daocloud.io/pages/viewpage.action?pageId=121145929 ###

| case id | category  | title | check point            | priority | status | other |
|---------|-----------|-------|------------------------|----------|--------|-------|
| E00001  | ipam-ip   | Single pod assign / releases ip addresses |1 check assign ip ok <br> 2 check release ip ok|        | done   |       |
| E00002  | ipam-ip |  two pods in one deployment  assign / releases ip addresses | | | | |
| E00003  | ipam-ip |  two pods in one statfulset  assign / releases ip addresses | | | | |
| E00004  | ipam-ip |  two pods in one damonset  assign / releases ip addresses   | | | | |
| E00005  | ipam-ip |  two pods in one job  assign / releases ip addresses | | | | |
| E00006  | ipam-ip |  two pods in one replicaset  assign / releases ip addresses | | | | |
| E00007  | ipam-ip | 128 pods in one deployment exclusive ip pool | | | | |
| E00008  | ipam-ip | ip allocation when ip pool is full | | | | |
| E00009  | gc | The CNI bin is removed and the GC is verified | | | | |
| E00010  | ipam-ip | ip release in forced deletion | | | | |
| E00011  | ipam-ip | Failed to create pod when IPv4 / IPv6 pool IP is exhausted <br>（optional） | | | | |
| E00012  | ipam-ip | When an invalid IPv4 or IPv6 pool is passed through the announcement mode | | | | |
| E00013  | ipam-ip | the IP address allocated is consistent with the address  by the CRD | | | | |
| E00014  | ipam-ip | After the namespace is deleted, the pod IP under it is recycled | | | | |
| E00015  | ipam-ip | After the IP address is released, it can be used again | | | | |
| E00016  | ipam-ip | Create a pod using long yaml | | | | |
| E00017  | ipam-ip pool | The IP pool is assigned to the namespace <br>（optional） | | | | |
| E00018  | ipam-ip pool |  The IP pool is assigned to pod <br>（optional） | | | | |
| E00019  | reliability | 256 pod start/ kill <br>（optional）| | | | |
| E00020  | reliability | IP allocation after restart of all hosts | | | | |
| E00021  | reliability | evited pod is drived out | | | | |
| E00022  | reliability | assign IP address when api-server/etcd starts and stops | | | | |

### judge whether to verify IPv4 and IPv6 according to the cluster environment ###