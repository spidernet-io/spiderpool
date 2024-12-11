# E2E Cases for Annotation

| Case ID | Title                                                                                                                       | Priority | Smoke | Status | Other |
| ------- |-----------------------------------------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| A00001  | It fails to run a pod with different VLANs for IPv4 and IPv6 IPPools                                                        | p3       |       | done   |       |
| A00002  | Added fields such as `"dist":"1.0.0.0/16"`, `"gw":"1.0.0.1"`, and `nics` and the pod was running successfully               | p2       |       | done   |       |
| A00003  | Failed to run a pod with invalid annotations                                                                                | p3       |       | done   |       |
| A00004  | Take a test with the Priority: pod annotation > namespace annotation > specified in a CNI profile                           | p1       |       | done   |       |
| A00005  | The "IPPools" annotation has the higher Priority over the "IPPool" annotation                                               | p1       |       | done   |       |
| A00006  | The namespace annotation has precedence over global default IPPool                                                          | p1       | true  | done   |       |
| A00008  | Successfully run an annotated multi-container pod                                                                           | p2       |       | done   |       |
| A00009  | Modify the annotated IPPool for a specified Deployment pod<br />Modify the annotated IPPool for a specified StatefulSet pod | p2       |       | done   |       |
| A00010  | Modify the annotated IPPool for a pod running on multiple NICs                                                              | p3       |       | done   |       |
| A00011  | Use the ippool route with `cleanGateway=false` in the pod annotation as a default route                                     | p3       |       | done   |       |
| A00012  | Specify the default route NIC through Pod annotation: `ipam.spidernet.io/default-route-nic`                                 | p2       |       | done   |       |
| A00013  | It's invalid to specify one NIC corresponding IPPool in IPPools annotation with multiple NICs                               | p2       |       | done   |       |
| A00014  | It's invalid to specify same NIC name for IPPools annotation with multiple NICs                                             | p2       |       | done   |       |
| A00015  | Use wildcard for 'ipam.spidernet.io/ippools' annotation to specify IPPools                                                  | p2       |       | done   |       |
| A00016 | In the annotation ipam.spidernet.io/ippools for multi-NICs, when the IP pool for one NIC runs out of IPs, it should not exhaust IPs from other pools. | p2 | | done | |
