# E2E Cases for coordinator

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| C00001  | coordinator in tuneMode: underlay works well | p1       |   smoke    | done   |       |
| C00002  | coordinator in tuneMode: overlay works well | p1      |  smoke  | done   |       |
| C00003  | coordinator in tuneMode: underlay with two NIC | p1      |  smoke  |    |       |
| C00004  | coordinator in tuneMode: overlay with two  NIC | p1      |  smoke  |    |       |
| C00005  | In overlay mode: specify the NIC where the default route is located | p2     |    |  done  |       |
| C00006  | In underlay mode: specify the NIC where the default route is located | p2     |    |       |       |
| C00007  | ip conflict detection (ipv4, ipv6) | p2     |    |  done  |       |
| C00008  | override pod mac prefix | p2       |       | done  |       |
| C00009  | gateway connection detection                  | p2     |    |  done  |       |
| C00010  | auto clean up the dirty rules(routing\neighborhood) while pod starting | p2 | | |
| C00011  | testing spidercoordinator.podCIDRType changes if works | p2 | | |
| C00012  | Support `spec.externalTrafficPolicy` for service in Local mode, it works well   | p2     |    |    |       |
| C00013  | Specify the NIC of the default route, but the NIC does not exist   | p3     |    |    |       |
| C00014  | Set hostRPFilter to `0, 1, 2`, rp_filter in host takes effect      | p3     |    |    |       |
| C00015  | Underlay or overlay can be automatically identified in auto mode   | p3     |    |    |       |
| C00016  | In multi-NIC mode, whether the NIC name is random and pods are created normally   | p3     |    |    |       |
| C00017  | If the value of `mode` is not underlay or overlay or auto, an error will be returned  | p3     |    |    |       |
| C00018  | mode is auto, use two different multus annotations to create  | p3     |    |    |       |
| C00019  | Set mode to disabled, coordinator will not be called  | p3     |    |    |       |
| C00020  | TunePodRoutes If false, no routing will be coordinated  | p3     |    |    |       |
| C00021  | The table can be customized by hostRuleTable            | p3     |    |    |       |
| C00022  | overwrite pod's interface name to a random name via multus and see if works           | p2     |    |    |       |
