# e2e case for edit crd

| case id   | title                                                                                             | priority | smoke | status | other |
|-----------|---------------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| D00001    | it fails to append an ip that already exists in another ippool to the ippool                      | p2       |       | NA     |       |
| D00002    | check the consistency between the revised ippool gateway, route etc. and the previous one         | p2       |       | NA     |       |
| D00003    | the wrong ippool gateway, route etc return error                                                  | p2       |       | NA     |       |
| D00004    | if the pod's ip in one ippool is occupied by the pod, it fails to delete the ippool, vice verse   | p2       |       | NA     |       |
| D00005    | if ippool/Spec/disabled in ipam is true, it fails to allocate ip from ippool, but de-allocates ip | p2       |       | NA     |       |
