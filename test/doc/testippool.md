# e2e case for ippool

| case id | category  | title                                 |check point | priority | status | other |
|---------|-----------|---------------------------------------|------------|----------|--------|-------|
| E00001  | assign ip | assign ip to a pod for case ipv4, ipv6, dual-stack ip || p1   | done   | In all use cases, do not create pods in the default namespace  |
| E00002  | assign ip | two pods in one deployment assign/releases ipv4, ipv6 addresses|| p1 | done ||
| E00003  | assign ip | two pods in one statefulSet assign/releases ipv4, ipv6 addresses|| p1 | done ||
| E00004  | assign ip | two pods in one daemon-Set assign/releases ipv4, ipv6 addresses||p1|NA||
| E00005  | assign ip | two pods in one job assign/releases ipv4, ipv6 addresses||p1|NA||
| E00006  | assign ip | two pods in one replica-Set assign/releases ipv4, ipv6 addresses||p1|NA||
| E00007  | assign ip | an invalid IPv4 or IPv6 pool passed in annotation mode cannot be assigned an IP address||p2|NA||
| E00008  | gc ip | after the namespace is deleted, the pod IP under the namespace is reclaimed ||p2|NA||
| E00009  | assign ip | create a pod using long yaml，assign / releases ipv4, ipv6 addresses ||p2|NA||
| E00010  | assign ip | annotation label for pod with init container or multiple containers ，check the container initialization and assign/releases ipv4, ipv6 addresses||p2|NA||
