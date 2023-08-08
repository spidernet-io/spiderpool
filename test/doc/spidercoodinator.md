# E2E Cases for spidercoordinator

| Case ID | Title                                                                    | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| V00001  | Switch podClusterType to other, whether the status changes as expected   | p3       |       |        |       |
| V00002  | Switch podClusterType to `none`, expect the cidr of status to be empty   | p3       |       |        |       |
| V00003  | status.phase is not-ready, expect the cidr of status to be empty         | p3       |       |        |       |
| V00004  | spidercoordinator has the lowest priority                                | p3       |       |        |       |
| V00005  | status.phase is not-ready, pods will fail to run                         | p3       |       |        |       |
