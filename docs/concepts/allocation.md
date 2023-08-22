# IP Allocation

When a pod is creating, it will follow steps below to get IP allocations.

1. Get all IPPool candidates.

    For which IPPool is used by a pod, the following rules are listed from **high to low priority** which means the previous rule would **override** the latter rule.

    * SpiderSubnet annotation. `ipam.spidernet.io/subnets` and `ipam.spidernet.io/subnet` will choose to use auto-created ippool if the SpiderSubnet feature is enabled. See [SpiderSubnet](../usage/spider-subnet.md) for details.
    * Honor pod annotation. `ipam.spidernet.io/ippools" and "ipam.spidernet.io/ippool` could be used to specify an ippool. See [Pod Annotation](../reference/annotation.md) for details.
    * Namespace annotation. `ipam.spidernet.io/defaultv4ippool` and `ipam.spidernet.io/defaultv6ippool` could be used to specify an ippool. See [namespace annotation](../reference/annotation.md) for details.
    * CNI configuration file. It can be set to `default_ipv4_ippool` and `default_ipv6_ippool` in the CNI configuration file. See [configuration](../reference/plugin-ipam.md) for details.
    * Cluster default IPPool. We can set SpiderIPPool CR object with `default` property, in which we'll regard it as a default pool in cluster.  See [configuration](../reference/crd-spiderippool.md) for details.

2. Filter valid IPPool candidates.

    After getting IPv4 and IPv6 IPPool candidates, it looks into each IPPool and figures out whether it meets following rules, and learns which candidate IPPool is available.

    * Filter terminating IPPools.
    * The `disable` field of the IPPool is `false`. This property means the IPPool is not available to be used.
    * Check `IPPool.Spec.NodeName` and `IPPool.Spec.NodeAffinity` properties whether match the scheduled node of the pod or not. If not match, this IPPool would be filtered. (`NodeName` has higher priority than `NodeAffinity`)
    * Check `IPPool.Spec.NamespaceName` and `IPPool.Spec.NamespaceAffinity` properties whether match the namespace of the pod or not. If not match, this IPPool would be filtered. (`NamespaceName` has higher priority than `NamespaceAffinity`)
    * The `IPPool.Spec.PodAffinity` field of the IPPool must meet the pod
    * Check `IPPool.Spec.MultusName` properties whether match the pod current NIC Multus configuration or not. If not match, this IPPool would be filtered.
    * The available IP resource of the IPPool is not exhausted

3. Sort IPPool candidates.

   We'll sort these IPPool candidates with our custom priority rules, then the IPAM prefers allocating IP addresses from the candidates in sequence.

   * The IPPool resource with `IPPool.Spec.PodAffinity` property has the highest priority.
   * The IPPool resource with `IPPool.Spec.NodeName` or `IPPool.Spec.NodeAffinity` property has the second-highest priority.
   * The IPPool resource with `IPPool.Spec.NamespaceName` or `IPPool.Spec.NamespaceAffinity` property has the second-highest priority.
   * The IPPool resource with `IPPool.Spec.MultusName` property has the lowest priority.
   
   > Notice: here are some simple instance to describe this rule.
   >
   > 1. *IPPoolA* with properties `IPPool.Spec.PodAffinity` and `IPPool.Spec.NodeName` has higher priority than *IPPoolB* with single affinity property `IPPool.Spec.PodAffinity`.
   > 2. *IPPoolA* with single property `IPPool.Spec.PodAffinity` has higher priority than *IPPoolB* with properties `IPPool.Spec.NodeName` and `IPPool.Spec.NamespaceName`.
   > 3. *IPPoolA* with properties `IPPool.Spec.PodAffinity` and `IPPool.Spec.NodeName` has higher priority than *IPPoolB* with properties `IPPool.Spec.PodAffinity`,`IPPool.Spec.NamespaceName` and `IPPool.Spec.MultusName`.

4. Assign IP from valid IPPool candidates.

    When trying to assign IP from the IPPool candidates, it follows rules as below.

    * The IP is not reserved by the "exclude_ips" field of the IPPool and all ReservedIP instances

> Notice: If the pod belongs to StatefulSet, it would be assigned IP addresses with the upper rules firstly. And it will try to reuse the last allocated IP addresses once the pod 'restarts'. 
