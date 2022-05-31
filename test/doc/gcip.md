# e2e case for gc ip

| case id   | title                                | priority | smoke |status | other |
|---------|---------------------------------------|------------|----------|--------|-------|
| G00001  | the garbage collection (gc) of the ip will be triggered after the namespace is deleted |p2||done||
| G00002  | set two pods with the same name in different namespace. one is immune to the fact that another is deleted  |p1 | | NA |  |
| G00003  | delete CNI/bin and then pod, ip will be in gc   |  p1   | |NA   |  |
| G00004  | the grace period=0 in deleting pod forces gc  |  p2 |  | NA   |  |
| G00005  | status/ip dirty data will be removed after a period of time|p2|   | NA   |  |
| G00006  | two different state of jobs: failed or succeed leads to the ip gc|  p2  | | NA   |  |
| G00007  | delete and reboot the spiderpool component in multi times. After the spiderpool recover, ip gc will be triggered|  p2 |  | NA   |  |
| G00008  | if the dirty data is occupied by pod, and the container id of the pod is invalid, ip gc will be triggered| p2| |NA |  |
