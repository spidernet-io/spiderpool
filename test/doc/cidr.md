# E2E Cases for CIDR

| Case ID | Title                                                                                                                               | Priority | Smoke | Status | Other |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------- | -------- | ----- | ------ | ----- |
| I00001  | Add, delete, modify, and query CIDR subnet                                                                                          |   p1     |       |  done  |       |
| I00002  | Add and remove IP for CIDR subnet                                                                                                   |   p1     |       |  done  |       |
| I00003  | Automatically create, scale, restart, and delete of ippools for different types of controllers                                      |   p1     |       |  done  |       |
| I00004  | Automatically create multiple ippools that can not use the same network segment and use IPs other than excludeIPs                   |   p2     |       |  done  |       |
| I00005  | If routes and gateway are modified for CIDR, how the manually created ippools are affected?                                         |   p3     |       |  done  |       |
| I00006  | Multiple automatic creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state |   p1     | true  |  done  |       |
| I00007  | Multiple manual creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state    |   p1     | true  |  done  |       |
| I00008  | Scale up and down the number of deployment several times to see if the state of the ippools is eventually stable                    |   p1     |       |  done  |       |
| I00009  | Create 100 Subnets with the same `Subnet.spec` and see if only one succeeds in the end                                              |   p1     |       |  done  |       |
| I00010  | Create 100 IPPools with the same `IPPool.spec` and see if only one succeeds in the end                                              |   p1     |       |  done  |       |
| I00011  | In ippool it should consider reservedIPs when subnets are assigned                                                                  |   p1     |       |        |       |

