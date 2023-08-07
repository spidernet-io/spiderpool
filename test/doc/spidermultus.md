# E2E Cases for spidermultus

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| M00001  | After creating spiderMultusConfig, a corresponding multus net-attach-conf will be created  | p1       |   smoke    | done   |       |
| M00002  | After deleting spiderMultusConfig, the corresponding net-attach-conf will also be deleted  | p1      |  smoke  | done   |       |
| M00003  | Update spidermultusConfig, the corresponding multus net-attach-conf will also be updated   | p1      |  smoke  |    |       |
| M00004  | Manually delete the net-attach-conf of multus, it will be created automatically | p1      |  smoke  |  done  |       |
| M00005  | Customize net-attach-conf name via annotation multus.spidernet.io/cr-name | p2       |       |  done  |       |
| M00006  | Change net-attach-conf version via annotation multus.spidernet.io/cni-version | p2     |       |  done  |       |
| M00007  | spidermultusconfig webhook verification, including vlan, cnitype and other fields | p3     |       |    |       |
