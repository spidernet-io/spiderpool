# E2E Cases for ReservedIP

| Case ID | Title                                                                                  | Priority | Smoke | Status | Other |
| ------- | ---------------------------------------------------------------------------------------| -------- | ----- | ------ | ----- |
| S00001  | An IP that is set in ReservedIP CRD should not be assigned to a pod                    | p2       |       | done   |       |
| S00002  | An IP that is set in the `excludeIPs` field of ippool, should not be assigned to a pod | p2       |       | done   |       |
| S00003  | Failed to set same IP in excludeIPs when an IP is assigned to a pod                    | p2       |       | done   |       |
| S00004  | Check excludeIPs for manually created ippools                                          | p3       |       | done   |       |
