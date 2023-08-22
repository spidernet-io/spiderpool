# E2E Cases for spidermultus

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| M00001  | testing creating spiderMultusConfig with cniType: macvlan and checking the net-attach-conf config if works | p1       |   smoke    | done   |       |
| M00002  | testing creating spiderMultusConfig with cniType: ipvlan and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00003  | testing creating spiderMultusConfig with cniType: sriov and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00004  | testing creating spiderMultusConfig with cniType: custom and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00005  | testing creating spiderMultusConfig with cniType: custom and invalid json config, expect error happened | p2       |       |    |       |
| M00006  | testing creating spiderMultusConfig with cniType: macvlan with vlanId with one master and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00007  | testing creating spiderMultusConfig with cniType: macvlan with vlanId with two master with bond config and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00008  | testing creating spiderMultusConfig with cniType: ovs with vlanId with and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00009  | testing creating spiderMultusConfig with cniType: ovs with trunk with and checking the net-attach-conf config if works | p1       |   smoke    |    |       |
| M00010  | testing creating spiderMultusConfig with cniType: ovs with device with and checking the net-attach-conf config if has annotations: ovs-cni.network.kubevirt.io/<brName> | p1       |   smoke    |    |       |
| M00011  | After deleting spiderMultusConfig, the corresponding net-attach-conf will also be deleted  | p2      |         |  done  |       |
| M00012  | Update spidermultusConfig, the corresponding multus net-attach-conf will also be updated   | p2      |         |        |       |
| M00013  | Update spidermultusConfig: add new bond config  | p1      |  smoke  |    |       |
| M00014  | Manually delete the net-attach-conf of multus, it will be created automatically | p1      |     |  done  |       |
| M00015  | Customize net-attach-conf name via annotation multus.spidernet.io/cr-name | p2       |       |    |       |
| M00016  | webhook validation for multus.spidernet.io/cr-name                        | p3       |       |    |       |
| M00017  | Change net-attach-conf version via annotation multus.spidernet.io/cni-version | p2     |       |    |       |
| M00018  | webhook validation for multus.spidernet.io/cni-version                        | p3       |       |    |       |
| M00019  | Set enableCoordinator to false, multus cr will not generate coordinator configuration | p3     |       |    |       |
| M00020  | Already have multus cr, spidermultus should take care of it                     | p3     |       |    |       |
| M00021  | Multiple annotations of spidermultus should be inherited by multus CR           | p3     |       |    |       |
| M00022  | The value of webhook verification cniType is inconsistent with cniConf          | p3     |       |    |       |
| M00023  | vlan is not in the range of 0-4094 and will not be created                    | p3     |       |    |       |
