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
