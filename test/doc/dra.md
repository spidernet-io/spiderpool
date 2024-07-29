# E2E Cases for DRA

| Case ID | Title                                                                             | Priority | Smoke | Status | Other |
| ------- | --------------------------------------------------------------------------------- | -------- | ----- | ------ | ----- |
| Q00001  | Creating a Pod to verify DRA if works while set rdmaAcc to true                                             | p1       | true  |  done  |       |
| Q00002  | Creating a Pod to verify DRA if works while set rdmaAcc to false                                             | p1       | true  |  done  |       |
| Q00003  | test dynamicNics with policy all | p3 | true |  |   | 
| Q00004  | create a pod with set defaultNic to nil, see the default cni what pod used is if the cluster default cni | p3 | true | |  |
| Q00005 | create a pod to testing the default route is from the secondaryNics | p3 | false | |  |
| Q00006 | create a pod to verify webhook auto inject the rdma resource to pod | p3 | false |  |  |
| Q00007 | create a pod to verify host'nic schedule for secondaryNics | p3 | false |  |  |
