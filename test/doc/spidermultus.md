# E2E Cases for spidermultus

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| M00001  | testing creating spiderMultusConfig with cniType: macvlan and checking the net-attach-conf config if works | p1       |   smoke    | done   |       |
| M00002  | testing creating spiderMultusConfig with cniType: ipvlan and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00003  | testing creating spiderMultusConfig with cniType: sriov and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00004  | testing creating spiderMultusConfig with cniType: custom and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00004  | testing creating spiderMultusConfig with cniType: custom and invalid json config, expect error happened | p1       |   smoke    |    |       |
| M00005  | testing creating spiderMultusConfig with cniType: macvlan with vlanId with one master and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00006  | testing creating spiderMultusConfig with cniType: macvlan with vlanId with two master with bond config and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00007  | After deleting spiderMultusConfig, the corresponding net-attach-conf will also be deleted  | p1      |  smoke  |    |       |
| M00008  | Update spidermultusConfig, the corresponding multus net-attach-conf will also be updated   | p1      |  smoke  |    |       |
| M00009  | Update spidermultusConfig: add new bond config  | p1      |  smoke  |    |       |
| M000010  | Manually delete the net-attach-conf of multus, it will be created automatically | p1      |  smoke  |    |       |
| M000011  | Customize net-attach-conf name via annotation multus.spidernet.io/cr-name | p2       |       |    |       |
| M000012 | Change net-attach-conf version via annotation multus.spidernet.io/cni-version | p2     |       |    |       |
