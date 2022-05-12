# e2e case for functional test

| case id | category  | title | check point            | priority | status | other |
|---------|-----------|-------|------------------------|----------|--------|-------|
| E00001  | ipam   | single pod assign / releases ip | 1 check assign ip ok <br> 2 check releases ip ok |        | done   |       |
| E00002  | ipam |  two pods in one deployment  assign / releases ip | | | | |
| E00003  | ipam |  two pods in one statefulset  assign / releases ip  | | | | |
| E00004  | ipam |  two pods in one damonset  assign / releases ip | | | | |
| E00005  | ipam |  two pods in one job  assign / releases ip | | | | |
| E00006  | ipam |  two pods in one replicaset  assign / releases ip | | | | |
| E00007  | ipam | 128 pods in one deployment exclusive ip pool | | | | |
| E00008  | ipam | ip allocation when ip pool is full | | | | |
| E00009  | gc | The CNI bin is removed and the GC is verified | | | | |
| E00010  | ipam | ip release in forced deletion | | | | |
| E00011  | ipam | Failed to create pod when IPv4 / IPv6 pool IP is exhausted <br>（optional） | | | | |
| E00012  | ipam | When an invalid IPv4 or IPv6 pool is passed through the announcement mode | | | | |
| E00013  | ipam | the IP address allocated is consistent with the address  by the crd | | | | |
| E00014  | ipam | After the namespace is deleted, the pod IP under it is recycled | | | | |
| E00015  | ipam | After the IP address is released, it can be used again | | | | |
| E00016  | ipam | Create a pod using long yaml | | | | |
| E00017  | ipam pool | The IP pool is assigned to the namespace <br>（optional） | | | | |
| E00018  | ipam pool |  The IP pool is assigned to pod <br>（optional） | | | | |
