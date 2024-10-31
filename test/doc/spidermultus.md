# E2E Cases for spidermultus

| Case ID | Title                                                                                                                                                   | Priority | Smoke | Status | Other |
|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------|----------|-------|--------| ----- |
| M00001  | testing creating spiderMultusConfig with cniType: macvlan and checking the net-attach-conf config if works                                              | p1       | smoke | done   |       |
| M00002  | testing creating spiderMultusConfig with cniType: ipvlan and checking the net-attach-conf config if works                                               | p1       | smoke | done   |       |
| M00003  | testing creating spiderMultusConfig with cniType: sriov and checking the net-attach-conf config if works                                                | p1       | smoke | done   |       |
| M00004  | testing creating spiderMultusConfig with cniType: custom and checking the net-attach-conf config if works                                               | p1       | smoke | done   |       |
| M00005  | testing creating spiderMultusConfig with cniType: custom and invalid json config, expect error happened                                                 | p2       |       | done   |       |
| M00006  | testing creating spiderMultusConfig with cniType: macvlan with vlanId with two master with bond config and checking the net-attach-conf config if works | p1       | smoke | done   |       |
| M00007  | Manually delete the net-attach-conf of multus, it will be created automatically                                                                         | p1       | smoke | done   |       |
| M00008  | After deleting spiderMultusConfig, the corresponding net-attach-conf will also be deleted                                                               | p2       |       | done   |       |
| M00009  | Update spidermultusConfig: add new bond config                                                                                                          | p1       | smoke | done   |       |
| M00010  | Customize net-attach-conf name via annotation multus.spidernet.io/cr-name                                                                               | p2       |       | done   |       |
| M00011  | webhook validation for multus.spidernet.io/cr-name                                                                                                      | p3       |       | done   |       |
| M00012  | Change net-attach-conf version via annotation multus.spidernet.io/cni-version                                                                           | p2       |       | done   |       |
| M00013  | webhook validation for multus.spidernet.io/cni-version                                                                                                  | p3       |       | done   |       |
| M00014  | Already have multus cr, spidermultus should take care of it                                                                                             | p3       |       | done   |       |
| M00015  | The value of webhook verification cniType is inconsistent with cniConf                                                                                  | p3       |       | done   |       |
| M00016  | vlan is not in the range of 0-4094 and will not be created                                                                                              | p3       |       | done   |       |
| M00017  | set disableIPAM to true and see if multus's nad has ipam config                                                                                         | p3       |       | done   |       |
| M00018  | set sriov.enableRdma to true and see if multus's nad has rdma config                                                                                    | p3       |       | done   |       |
| M00019  | set spidermultusconfig.spec to empty and see if works                                                                                                   | p3       |       | done   |       |
| M00020  | annotating custom names that are too long or empty should fail                                                                                          | p3       |       | done   |       |
| M00021  | create a spidermultusconfig and pod to verify chainCNI json config if works                                                                             | p3       |       | done   |       |
| M00022  | test podRPFilter and hostRPFilter in spidermultusconfig                                                                                                 | p3       |       | done   |       |
| M00023  | set hostRPFilter and podRPFilter to a invalid value                                                                                                     | p3       |       | done   |       |
| M00024  | verify the podMACPrefix filed                                                                                                                           | p3       |       | done   |       |
| M00025  | The custom net-attach-conf name from the annotation multus.spidernet.io/cr-name doesn't follow Kubernetes naming rules and can't be created.            | p3       |       | done   |       |
| M00026  |    check the coordinatorConfig: enableVethLinkLocakAddress works                                                                                     | p3       |       | done   |       |
| M00027  |    rdma must be enabled and ippools config must be set when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject                                                                                     | p3       |       | done   |       |
| M00028  |    return an err if rdma is not enabled when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject                                                                                     | p3       |       | done   |       |
| M00029  |    return an err if no ippools config when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject                                                                                     | p3       |       | done   |       |
| M00030  |    return an err if cniType is not in [macvlan,ipvlan,sriov,ib-sriov,ipoib] when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject                                                                                     | p3       |       | done   |       |
