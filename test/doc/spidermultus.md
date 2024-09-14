# E2E Cases for spidermultus

| Case ID | Title                                                                                                                                                   | Priority | Smoke | Status | Other |
|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------| -------- |-------| ------ | ----- |
| M00001  | testing creating spiderMultusConfig with cniType: macvlan and checking the net-attach-conf config if works                                              | p1       | smoke | done   |       |
| M00002  | testing creating spiderMultusConfig with cniType: ipvlan and checking the net-attach-conf config if works                                               | p1       | smoke |   done |       |
| M00003  | testing creating spiderMultusConfig with cniType: sriov and checking the net-attach-conf config if works                                                | p1       | smoke |   done  |       |
| M00004  | testing creating spiderMultusConfig with cniType: custom and checking the net-attach-conf config if works                                               | p1       | smoke |  done  |       |
| M00005  | testing creating spiderMultusConfig with cniType: custom and invalid json config, expect error happened                                                 | p2       |       |  done  |       |
| M00007  | testing creating spiderMultusConfig with cniType: macvlan with vlanId with two master with bond config and checking the net-attach-conf config if works | p1       | smoke |    |       |
| M00011  | After deleting spiderMultusConfig, the corresponding net-attach-conf will also be deleted                                                               | p2      |       |  done  |       |
| M00013  | Update spidermultusConfig: add new bond config                                                                                                          | p1      | smoke |    |       |
| M00014  | Manually delete the net-attach-conf of multus, it will be created automatically                                                                         | p1      |       |  done  |       |
| M00015  | Customize net-attach-conf name via annotation multus.spidernet.io/cr-name                                                                               | p2       |       |  done  |       |
| M00016  | webhook validation for multus.spidernet.io/cr-name                                                                                                      | p3       |       |  done  |       |
| M00017  | Change net-attach-conf version via annotation multus.spidernet.io/cni-version                                                                           | p2     |       |   done  |       |
| M00018  | webhook validation for multus.spidernet.io/cni-version                                                                                                  | p3       |       |  done  |       |
| M00020  | Already have multus cr, spidermultus should take care of it                                                                                             | p3     |       |  done  |       |
| M00022  | The value of webhook verification cniType is inconsistent with cniConf                                                                                  | p3     |       | done |       |
| M00023  | vlan is not in the range of 0-4094 and will not be created                                                                                              | p3     |       |  done  |       |
| M00024  | set disableIPAM to true and see if multus's nad has ipam config                                                                                         | p3     |       |  done  |       |
| M00025  | set sriov.enableRdma to true and see if multus's nad has rdma config                                                                                    | p3     |       |    |       |
| M00026  | set spidermultusconfig.spec to empty and see if works                                                                                                   | p3     |       |    |       |
| M00027 | test podRPFilter and hostRPFilter in spidermultusconfig | p3 | | done |
| M00028 | set hostRPFilter and podRPFilter to a invalid value | p3 | | done |
