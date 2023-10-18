# E2E Cases for spidercoordinator

| Case ID | Title                                                                    | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| V00001  | Switch podClusterType to `auto`, see if it could auto fetch the type       | p3       |       |        |       |
| V00002  | Switch podClusterType to `auto` but no cni files in /etc/cni/net.d, see if the phase is NotReady       | p3       |       |        |       |
| V00003  | Switch podClusterType to `calico`, see if it could auto fetch the cidr from calico ippools   | p3       |       |        |       |
| V00004  | Switch podClusterType to `cilium`, see if it works in ipam-mode: [cluster-pool,kubernetes,multi-pool]   | p3       |       |        |       |
| V00005  | Switch podClusterType to `none`, expect the cidr of status to be empty   | p3       |       |        |       |
| V00006  | status.phase is not-ready, expect the cidr of status to be empty         | p3       |       |        |       |
| V00007  | spidercoordinator has the lowest priority                                | p3       |       |        |       |
| V00008  | status.phase is not-ready, pods will fail to run                         | p3       |       |        |       |
