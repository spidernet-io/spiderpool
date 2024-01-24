# E2E Cases for spidercoordinator

| Case ID | Title                                                                                                     | Priority | Smoke | Status | Other |
| ------- | --------------------------------------------------------------------------------------------------------- | -------- | ----- | ------ | ----- |
| V00001  | Switch podCIDRType to `auto`, see if it could auto fetch the type                                         | p3       |       |  done  |       |
| V00002  | Switch podCIDRType to `auto` but no cni files in /etc/cni/net.d, Viewing should be consistent with `none` | p3       |       |  done  |       |
| V00003  | Switch podCIDRType to `calico`, see if it could auto fetch the cidr from calico ippools                   | p3       |       |  done  |       |
| V00004  | Switch podCIDRType to `cilium`, see if it works in ipam-mode: [cluster-pool,kubernetes,multi-pool]        | p3       |       |  done  |       |
| V00005  | Switch podCIDRType to `none`, expect the cidr of status to be empty                                       | p3       |       |  done  |       |
| V00006  | status.phase is not-ready, expect the cidr of status to be empty                                          | p3       |       |  done  |       |
| V00007  | spidercoordinator has the lowest priority                                                                 | p3       |       |  done  |       |
| V00008  | status.phase is not-ready, pods will fail to run                                                          | p3       |       |  done  |       |
| V00009 | it can get the clusterCIDR from kubeadmConfig or kube-controller-manager pod                               | p3       |       |  done  |       |
