# IP Allocation

when a pod is creating, it observes following steps to assign ip:

1. get all ippool candidates

    For which ippool is used by pod, the following rule is listed from high to low priority

    * Honor pod annotation. "ipam.spidernet.io/ippool" and "ipam.spidernet.io/ippools" could be used for specify ippool. It could refer to [pod annotation](./annotation.md) for detail.

    * Namespace annotation. "ipam.spidernet.io/defaultv4ippool" and "ipam.spidernet.io/defaultv6ippool" could be used for specify ippool. It could refer to [namespace annotation](./annotation.md) for detail.

    * Cluster default ippool.
    Cluster default ippool could be set to "clusterDefaultIpv4Ippool" and "clusterDefaultIpv6Ippool" in configmap "spiderpool-conf". It could refer to [configuration](./config.md) for detail.

2. filter valid ippool candidate

    after getting ipv4 ippool and ipv6 ippool candidates, it looks into each ippool and figure out whether it meets following rules, and get which candidate ippool is available

    * the "disable" filed of the ippool is "false"
    * the "ipversion" filed of the ippool must meet the claim
    * the "namespaceSelector" filed of the ippool must meet the namespace of the pod
    * the "podSelector" filed of the ippool must meet the pod
    * the "nodeSelector" filed of the ippool must meet the scheduled node of the pod
    * the available IP resource of the ippool is not exhausted

3. assign ip from valid ippool candidate

    when trying to assign IP from the candidate ippool, it follows below rules

    * the IP is not reserved by the "exclude_ips" filed of the ippool and all ReservedIP instance
    * when the controller of the pod is statefulset, it allocates IP by order
