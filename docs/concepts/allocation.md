# IP Allocation

When a pod is creating, it will follow steps below to get an IP.

1. Get all ippool candidates.

    For which ippool is used by a pod, the following rules are listed from high to low priority.

    * Honor pod annotation. "ipam.spidernet.io/ippool" and "ipam.spidernet.io/ippools" could be used to specify an ippool. See [Pod Annotation](../usage/annotation.md) for detail.

    * Namespace annotation. "ipam.spidernet.io/defaultv4ippool" and "ipam.spidernet.io/defaultv6ippool" could be used to specify an ippool. See [namespace annotation](../usage/annotation.md) for detail.

    * Cluster default ippool.
      It can be set to "clusterDefaultIPv4IPPool" and "clusterDefaultIPv6IPPool" in the "spiderpool-conf" ConfigMap. See [configuration](../usage/config.md) for detail.

2. Filter valid ippool candidates.

    After getting IPv4 and IPv6 ippool candidates, it looks into each ippool and figures out whether it meets following rules, and learns which candidate ippool is available.

    * The "disable" field of the ippool is "false"
    * The "ipversion" field of the ippool must meet the claim
    * The "namespaceSelector" field of the ippool must meet the namespace of the pod
    * The "podSelector" field of the ippool must meet the pod
    * The "nodeSelector" field of the ippool must meet the scheduled node of the pod
    * The available IP resource of the ippool is not exhausted

3. Assign IP from valid ippool candidates.

    When trying to assign IP from the ippool candidates, it follows rules as below.

    * The IP is not reserved by the "exclude_ips" field of the ippool and all ReservedIP instances
    * When the pod controller is a StatefulSet, the pod will get an IP in sequence
