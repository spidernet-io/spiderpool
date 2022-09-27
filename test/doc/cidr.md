# E2E Cases for CIDR

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| I00001  | Add, delete, modify, and query CIDR subnet                   |   p1     |       |        |       |
| I00002  | Add and remove IP for CIDR subnet                            |   p1     |       |        |       |
| I00003  | Automatically create ippool, and create, scale, restart, and delete a controller |    p1    |       |         |       |
| I00004  | Automatically create multiple ippools that can not use the same network segment and use IPs other than excludeIPs |    p2    |       |          |       |
| I00005  | If routes and gateway are modified for CIDR, how the manually created ippools are affected? |   p3     |         |        |       |
| I00006  | Multiple automatic creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state |   p1     |       |        |       |
| I00007  | Multiple manual creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state |   p1     |       |        |       |
