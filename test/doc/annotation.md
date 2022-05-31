# e2e case for annotation

| case id | title                             |priority | smoke | status | other |
|---------|-----------------------------------------|-------------|--------|----------|--------|
| A00001  | it will fail to run pod with different VLAN for ipv4 and ipv6 ippool | p2 || NA | |
| A00002  | different NICS can use different pools and success to run pod|p2||NA||
| A00003  | invalid annotations will contribute to pod run error |p2||NA||
| A00004  | the pod annotation has the highest priority over namespace and global default|p2||NA||
| A00005  | ippools has the highest priority over ippool|p2||NA||
