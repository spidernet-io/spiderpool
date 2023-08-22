# E2E Cases for ifacer

| Case ID | Title                                                                             | Priority | Smoke | Status | Other |
| ------- | --------------------------------------------------------------------------------- | -------- | ----- | ------ | ----- |
| N00001  | Creating a VLAN interface should succeed                                          | p1       | true  |        |       |
| N00002  | VLAN interface already exists, skip creation                                      | p2       |       |        |       |
| N00003  | VLAN interface exists but state is down, set it up and exit                       | p2       |       |        |       |
| N00004  | Different VLAN interfaces have the same VLAN id, an error is returned             | p2       |       |        |       |
| N00005  | The master interface is down, setting it up and creating VLAN interface           | p2       |       |        |       |
| N00006  | Restart the node vlan/bond will be lost, restart the pod they should be restored. | p3       |       |        |       |
