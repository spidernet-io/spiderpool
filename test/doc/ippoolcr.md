# E2E Cases for IPPool CR

| Case ID | Title                                                        | Priority | Smoke | Status | Other |
| ------- | ------------------------------------------------------------ | -------- | ----- | ------ | ----- |
| D00001  | An IPPool fails to add an IP that already exists in an other IPPool | p2       |       | done   |       |
| D00002  | Add a route with `routes` and `gateway` fields in the ippool spec, which only takes effect on the new pod and does not on the old pods | p2       |       | done   |       |
| D00003  | Failed to add wrong IPPool gateway and route to an IPPool CR | p2       |       | done   |       |
| D00004  | Failed to delete an IPPool whose IP is not de-allocated at all | p2       |       | done   |       |
| D00005  | A "true" value of IPPool/Spec/disabled should forbid IP allocation, but still allow ip de-allocation | p2       |       | done   |       |
| D00006  | Successfully create and delete IPPools in batch              | p2       |       | done   |       |
| D00007  | Modify `routes` in the ippool spec to disallow using `0.0.0.0/0` |          |       |        |       |
