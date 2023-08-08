# E2E Cases for IPPool CR

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| D00001  | An IPPool fails to add an IP that already exists in an other IPPool | p2       |       | done   |       |
| D00002  | Add a route with `routes` and `gateway` fields in the ippool spec, which only takes effect on the new pod and does not on the old pods | p2       |  smoke  | done   |       |
| D00003  | Failed to add wrong IPPool gateway and route to an IPPool CR | p2       |       | done   |       |
| D00004  | Failed to delete an IPPool whose IP is not de-allocated at all | p2     |       | done   |       |
| D00005  | A "true" value of IPPool/Spec/disabled should forbid IP allocation, but still allow ip de-allocation | p2       |       | done   |       |
| D00006  | Successfully create and delete IPPools in batch                  | p2      |       | done   |       |
| D00007  | Add, delete, modify, and query ippools that are created manually | p1      |       | done   |       |
| D00008  | Manually ippool inherits subnet attributes (including routes, vlanId, etc.) | p3      |       |     |       |
| D00009  | multusName matches, IP can be assigned                | p2      |       |     |       |
| D00010  | multusName mismatch, unable to assign IP              | p3      |       |     |       |
| D00011  | The node where the pod is located matches the nodeName, and the IP can be assigned  | p2      |       |     |       |
| D00012  | The node where the pod resides does not match the nodeName, and the IP cannot be assigned  | p3      |       |     |       |
| D00013  | nodeName has higher priority than nodeAffinity        | p3      |       |     |       |
| D00014  | The namespace where the pod is located matches the namespaceName, and the IP can be assigned     | p2      |       |     |       |
| D00015  | The namespace where the pod resides does not match the namespaceName, and the IP cannot be assigned      | p2      |       |     |       |
| D00016  | namespaceName has higher priority than namespaceAffinity                                | p3      |       |     |       |
| D00017  | Multiple default IP pools can be set by `default: true`          | p3      |       |     |       |
| D00018 | Specify multiple multus cr through `k8s.v1.cni.cncf.io/networks` annotation to realize multi-nics function | p3     |       |    |       |
