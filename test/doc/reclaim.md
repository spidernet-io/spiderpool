# E2E Cases for Reclaim IP

| Case ID | Title                                                                                                                                      | Priority | Smoke | Status | Other |
|---------|--------------------------------------------------------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| G00001  | Related IP resource recorded in IPPool will be reclaimed after the namespace is deleted                                                    | p1       | true  | done   |       |
| G00002  | The IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted                              | p1       |       | done   |       |
| G00003  | The IP can be reclaimed after its deployment, statefulset, daemonset, replica, or job is deleted, even when CNI binary is gone on the host | p1       |       | done   |       |
| G00004  | The IP should be reclaimed when deleting the pod with 0 second of grace period                                                             | p2       |       | done   |       |
| G00005  | A dirty IP record (pod name is wrong) in the IPPool should be auto clean by Spiderpool                                                     | p2       |       | done   |       |
| G00006  | The IP should be reclaimed for the job pod finished with success or failure Status                                                         | p2       |       | done   |       |
| G00007  | A dirty IP record (pod name is right but container ID is wrong) in the IPPool should be auto clean by Spiderpool                           | p3       |       | done   |       |
| G00008  | The Spiderpool component recovery from repeated reboot, and could correctly reclaim IP                                                     | p3       |       | done   |       |
| G00009  | stateless workload IP could be released with node not ready                                                                                | p3       |       | done   |       |
| G00010  | IP addresses not used by statefulSet can be released by gc all ready                                                                       | p3       |       | done   |       |
| G00011  | The IPPool is used by 2 statefulSets and scaling up/down the replicas, gc works normally and there is no IP conflict in statefulset.       | p2       |       | done   |       |
| G00012  | Multiple resource types compete for a single IPPool. In scenarios of creation, scaling up/down, and deletion, GC all can correctly handle IP addresses.       | p2       |       | done   |       |
