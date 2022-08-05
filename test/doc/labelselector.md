# E2E Cases for Label Selector

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| L00001  | Successfully run deployment/pod who is bound to an IPPool set with matched nodeSelector | p2       | true  | NA     |       |
| L00002  | Failed to run deployment/pod who is bound to an IPPool set with no-matched nodeSelector | p3       |       | NA     |       |
| L00003  | Successfully run deployment/pod who is bound to an IPPool set with matched namespaceSelector | p2       | true  | NA     |       |
| L00004  | Failed to run deployment/pod who is bound to an IPPool set with no-matched namespaceSelector | p3       |       | NA     |       |
| L00005  | Successfully run deployment/pod who is bound to an IPPool set with matched podSelector | p2       | true  | NA     |       |
| L00006  | Failed to run deployment/pod who is bound to an IPPool set with no-matched podSelector | p3       |       | NA     |       |
| L00007  | Successfully run deployment/pod who is cross-zone deployment with matched nodeSelector | p2       |       | NA     |       |
| L00008  | Successfully restarted statefulSet/pod with matching podSelector, ip remains the same | p2       |       | NA     |       |
| L00009  | Multiple IPPools can be used in the same namespace and one IPPool can be used by multiple namespace | p2       |       | NA     |       |
