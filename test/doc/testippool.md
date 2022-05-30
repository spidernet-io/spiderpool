# e2e case for ippool

| case id | category  | title                                 |check point | priority | status | other |
|---------|-----------|---------------------------------------|------------|----------|--------|-------|
| E00001  | assign ip | assign ip to a pod for case ipv4, ipv6, dual-stack ip || p1   | done   | In all use cases, do not create pods in the default namespace  |
| E00002  | assign ip | two pods in one deployment assign/releases ipv4, ipv6 addresses|| p1 | done ||
| E00003  | assign ip | two pods in one statefulSet assign/releases ipv4, ipv6 addresses||p1|NA||
| E00004  | assign ip | two pods in one daemon-Set assign/releases ipv4, ipv6 addresses||p1|NA||
| E00005  | assign ip | two pods in one job assign/releases ipv4, ipv6 addresses||p1|NA||
| E00006  | assign ip | two pods in one replica-Set assign/releases ipv4, ipv6 addresses||p1|NA||
| E00007  | assign ip | an invalid IPv4 or IPv6 pool passed in annotation mode cannot be assigned an IP address||p2|NA||
| E00008  | gc ip | after the namespace is deleted, the pod IP under the namespace is reclaimed ||p2|NA||
| E00009  | assign ip | create a pod using long yaml，assign / releases ipv4, ipv6 addresses ||p2|NA||
| E00010  | assign ip | annotation label for pod with init container or multiple containers ，check the container initialization and assign/releases ipv4, ipv6 addresses||p2|NA||
| E00011  | gc ip | Assign ippool to cluster via label selector（node selector） |||NA||
| E00012  | assign ip | Assign ippool to namespace via label selector（namespace selector） |||NA||
| E00013  | assign ip | Assign ippool to pod via label selector（pod selector）|||NA||
| E00014  | assign ip | If the Spec/disabled in ipam is true, CNI cannot allocate ip from ippool, but can still de-allocate ip|||NA||
| E00015  | assign ip | Concurrency scenario, perform add/delete IPS IP, check related information |||NA||
| E00016  | assign ip | Concurrent scenarios, perform add/remove exclude IPS, check related information|||NA||
| E00017  | assign ip | gateway ip modification, the modified ip value must be in the SUBNET |||NA||
| E00018  | assign ip | ipam change routes rule, only for new pods, stock pods should not be affected |||NA||
| E00019  | assign ip | In a MULTI NIC scenario, the NIC is assigned via pod annotation|||NA||
| E00020  | assign ip | Specify multi-ipv4, ipv6 ippool at tenant level via namespace annotation|||NA||
| E00021  | ip detect | ip conflict detection mechanism under the same namespace |||NA||
| E00022  | ip detect | ip conflict detection mechanism under different namespace|||NA||
| E00023  | assign ip | In the multus scenario, different NICS can use different pools|||NA||
| E00024  | assign ip | Support ipv4-only / ipv6-only / ipv4-ipv6 change|||NA||
| E00025  | assign ip | When ippool does CRUD, check the changes of allocate,de-allocate,total count as expected |||NA||
| E00026  | assign ip | The determination range of VLAN value should be consistent with 0-4094, for ipv4 and ipv6 VLAN value should be consistent|||NA||
| E00027  | assign ip | Assignment and deletion of ip for applications of the same name in different namespace|||NA||
| E00028  | assign ip | Restart and new operation for the pod with assigned ip, check ip fixed, no change|||NA||
| E00029  | assign ip | Redefine routing rules through pod annotation|||NA||
| E00030  | ip detect | The same ip is assigned to different pods multiple times to check for ip conflicts|||NA||
