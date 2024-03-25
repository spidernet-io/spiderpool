# E2E Cases for SpiderSubnet

| Case ID | Title                                                                                                                               | Priority | Smoke | Status  | Other |
|---------|-------------------------------------------------------------------------------------------------------------------------------------|----------|-------|---------|-------|
| I00001  | Add, delete, modify, and query CIDR subnet                                                                                          | p1       |       | done    |       |
| I00002  | Add and remove IP for CIDR subnet                                                                                                   | p1       |       | done    |       |
| I00003  | Automatically create, scale, restart, and delete of ippools for different types of controllers                                      | p1       |       | done    |       |
| I00004  | The excludeIPs in the subnet will not be used by pools created automatically or manually.                                           | p2       |       | done    |       |
| I00005  | If routes and gateway are modified for CIDR, how the manually created ippools are affected?                                         | p3       |       | done    |       |
| I00006  | Multiple automatic creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state | p1       | true  | done    |       |
| I00007  | Multiple manual creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state    | p1       | true  | done    |       |
| I00008  | Scale up and down the number of deployment several times to see if the state of the ippools is eventually stable                    | p1       |       | done    |       |
| I00009  | Create 100 Subnets with the same `Subnet.spec` and see if only one succeeds in the end                                              | p1       |       | done    |       |
| I00010  | Create 100 IPPools with the same `IPPool.spec` and see if only one succeeds in the end                                              | p1       |       | done    |       |
| I00011  | The subnet automatically creates an ippool and allocates IP, and should consider reservedIP                                         | p1       |       | done    |       |
| I00012  | SpiderSubnet supports multiple interfaces                                                                                           | p1       |       | done    |       |
| I00013  | SpiderSubnet supports automatic IP assignment for Pods                                                                              | p2       |       | done    |       |
| I00014  | The default subnet should support different controller types                                                                        | p2       |       | done    |       |
| I00015  | Applications with the same name and type can use the reserved IPPool.                                                               | p2       |       | done    |       |
| I00016  | Applications with the same name and different types cannot use the reserved IPPool.                                                 | p3       |       | done    |       |
| I00017  | Ability to create fixed IPPools for applications with very long names                                                               | p3       |       | done    |       |
| I00018  | automatic IPPool IPs are not modifiable                                                                                             | p3       |       | done    |       |
| I00019  | Change the annotation ipam.spidernet.io/ippool-reclaim to true and the reserved IPPool will be reclaimed.                           | p3       |       | done    |       |
| I00020  | Redundant IPs for automatic IPPool, which cannot be used by other applications                                                      | p3       |       | done    |       |
| I00021  | Pod works correctly when multiple NICs are specified by annotations for applications of the same name                               | p3       |       | done    |       |
| I00022  | Dirty data in the subnet should be recycled.                                                                                        | p3       |       | done    |       |
| I00023  | SpiderSubnet feature doesn't support orphan pod                                                                                     | p2       |       | done    |       |
| I00024  | The Pod would not be setup and no auto Pools created when the SpiderSubnet AutoPool feature is disabled                             | p2       |       | done    |       |
