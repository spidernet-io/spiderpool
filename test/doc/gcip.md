# e2e case for reclaim ip

| case id | title                                                                                                           | priority  | smoke | status | other |
|---------|-----------------------------------------------------------------------------------------------------------------|-----------|-------|--------|-------|
| G00001  | related IP resource recorded in ippool will be reclaimed after the namespace is deleted                         | p2        |       | done   |       |
| G00002  | the IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted   | p1        |       | NA     |       |
| G00003  | the IP should be reclaimed after its pod is deleted , even when CNI binary is gone on the host                  | p1        |       | NA     |       |
| G00004  | the IP should be reclaimed when deleting the pod with 0 second of grace period                                  | p2        |       | NA     |       |
| G00005  | a dirty IP record (pod name is wrong) in the ippool should be auto clean by spiderpool                           | p2        |       | NA     |       |
| G00006  | the IP should be reclaimed for the job pod finished with success or failure status                              | p2        |       | NA     |       |
| G00007  | a dirty IP record (pod name is right but container ID is wrong) in the ippool should be auto clean by spiderpool | p2        |       | NA     |       |
| G00008  | the spiderpool component recovery from repeated reboot, and could correctly reclaim IP                          | p2        |       | NA     |       |
