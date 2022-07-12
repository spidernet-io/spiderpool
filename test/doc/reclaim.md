# e2e case for reclaim ip

| case id | title                                                                                                            | priority  | smoke | status | other |
|---------|------------------------------------------------------------------------------------------------------------------|-----------|-------|--------|-------|
| G00001  | related IP resource recorded in IPPool will be reclaimed after the namespace is deleted                          | p1        | true  | done   |       |
| G00002  | the IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted    | p1        |       | done   |       |
| G00003  | the IP should be reclaimed after its pod is deleted, even when CNI binary is gone on the host                    | p1        |       | NA     |       |
| G00004  | the IP should be reclaimed when deleting the pod with 0 second of grace period                                   | p2        |       | NA     |       |
| G00005  | a dirty IP record (pod name is wrong) in the IPPool should be auto clean by Spiderpool                           | p2        |       | NA     |       |
| G00006  | the IP should be reclaimed for the job pod finished with success or failure status                               | p2        |       | NA     |       |
| G00007  | a dirty IP record (pod name is right but container ID is wrong) in the IPPool should be auto clean by Spiderpool | p3        |       | NA     |       |
| G00008  | the Spiderpool component recovery from repeated reboot, and could correctly reclaim IP                           | p3        |       | NA     |       |
