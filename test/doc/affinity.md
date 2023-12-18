# E2E Cases for Label Selector

| Case ID | Title                                                                                               | Priority | Smoke | Status | Other |
| ------- |-----------------------------------------------------------------------------------------------------| -------- | ----- | ------ | ----- |
| L00001  | Successfully run deployment/pod that is bound to an IPPool set with matched `NodeAffinity`          | p2       | true  | done   |       |
| L00002  | Failed to run deployment/pod that is bound to an IPPool set with no-matched `NodeAffinity`          | p3       |       | done   |       |
| L00003  | Successfully run deployment/pod that is bound to an IPPool set with matched `NamespaceAffinity`     | p2       | true  | done   |       |
| L00004  | Failed to run deployment/pod that is bound to an IPPool set with no-matched `NamespaceAffinity`     | p3       |       | done   |       |
| L00005  | Successfully run deployment/pod that is bound to an IPPool set with matched `PodAffinity`           | p2       | true  | done   |       |
| L00006  | Failed to run deployment/pod that is bound to an IPPool set with no-matched `PodAffinity`           | p3       |       | done   |       |
| L00007  | Successfully run daemonSet/pod that is cross-zone daemonSet with matched `NodeAffinity`             | p2       |       | done   |       |
| L00008  | Successfully restarted statefulSet/pod with matching `PodAffinity`, ip remains the same             | p2       |       | done   |       |
| L00009  | Multiple IPPools can be used in the same namespace and one IPPool can be used by multiple namespace | p2       |       | done   |       |
