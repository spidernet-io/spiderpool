# e2e case for assign ip

| case id   | title                                 |priority | smoke | status | other |
|---------|---------------------------------------|------------|----------|--------|-------|
| E00001  | assign ip to a pod for case ipv4, ipv6 and dual-stack ip |p1 | true | done   | |
| E00002  | assign ip to a pod for case ipv4, ipv6 and dual-stack ip through deployment|p1| true | done ||
| E00003  | assign ip to a pod for case ipv4, ipv6 and dual-stack ip through statefulSet|p1|true|done||
| E00004  | assign ip to a pod for case ipv4, ipv6 and dual-stack ip through daemon-Set|p1|true|done||
| E00005  | assign ip to a pod for case ipv4, ipv6 and dual-stack ip through one job|p1|true|NA||
| E00006  | assign ip to a pod for case ipv4, ipv6 and dual-stack ip through one replica-Set|p1|true|done||
| E00007  | create a pod in long yaml，and then assign ip to a pod for case ipv4, ipv6 addresses |p2||done||
| E00008  | set ippool/spec/node selector to assign node and success to run pod|p2||NA||
| E00009  | set ippool/spec/node selector to assign node and fail to run pod |p3||NA||
| E00010  | set ippool/spec/namespace selector to assign namespace and success to run pod |p2||NA||
| E00011  | set ippool/spec/namespace selector to assign namespace and fail to run pod |p3||NA||
| E00012  | set ippool/spec/pod selector to assign pod and fail to run pod|p2||NA||
| E00013  | set ippool/spec/pod selector to assign pod and success to run pod|p3||NA||
| E00014  | create and delete 1000 ippool successfully|p2||NA||
| E00015  |fail to run pod when ip in ippool is exhausted|p3||NA||
