# E2E Cases for coordinator

| Case ID | Title                                                                                                                                                         | Priority | Smoke | Status | Other |
|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| C00001  | coordinator in tuneMode: underlay works well                                                                                                                  | p1       | smoke | done   |       |
| C00002  | coordinator in tuneMode: overlay works well                                                                                                                   | p1       | smoke | done   |       |
| C00003  | coordinator in tuneMode: underlay with two NIC                                                                                                                | p1       | smoke | done   |       |
| C00004  | coordinator in tuneMode: overlay with two  NIC                                                                                                                | p1       | smoke | done   |       |
| C00005  | In overlay mode: specify the NIC (net1) where the default route is located, use 'ip r get 8.8.8.8' to see if default route nic is the specify NIC             | p2       |       | done   |       |
| C00006  | In underlay mode: specify the NIC (net1) where the default route is located, use 'ip r get 8.8.8.8' to see if default route nic is the specify NIC            | p2       |       | done   |       |
| C00007  | ip conflict detection (ipv4, ipv6)                                                                                                                            | p2       |       | done   |       |
| C00008  | override pod mac prefix                                                                                                                                       | p2       |       | done   |       |
| C00009  | gateway connection detection                                                                                                                                  | p2       |       | done   |       |
| C00010  | auto clean up the dirty rules(routing\neighborhood) while pod starting                                                                                        | p2       |       | done   |       |
| C00011  | In the default scenario (Do not specify the NIC where the default route is located in any way) , use 'ip r get 8.8.8.8' to see if default route NIC is `eth0` | p2       |       | done   |       |
| C00012  | In multi-nic case , use 'ip r get <service_subnet> and <hostIP>' to see if src is from pod's eth0, note: only for ipv4.                                       | p2       |       | done   |       |
| C00013  | Support `spec.externalTrafficPolicy` for service in Local mode, it works well                                                                                 | p2       |       | done   |       |
| C00014  | Specify the NIC of the default route, but the NIC does not exist                                                                                              | p3       |       | done   |       |
| C00015  | In multi-NIC mode, whether the NIC name is random and pods are created normally                                                                               | p3       |       | done   |       |
| C00016  | The table name can be customized by hostRuleTable                                                                                                             | p3       |       | done   |       |
| C00017  | TunePodRoutes If false, no routing will be coordinated                                                                                                        | p3       |       | done   |       |
| C00018  | The conflict IPs for stateless Pod should be released                                                                                                         | p3       |       | done   |       |
| C00019  | The conflict IPs for stateful Pod should not be released                                                                                                      | p3       |       | done   |       |
