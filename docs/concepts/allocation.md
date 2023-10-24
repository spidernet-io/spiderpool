# IP Allocation

**English** | [**简体中文**](./allocation-zh_CN.md)

When creating a Pod, it will follow the steps below to get IP allocations.The lifecycle of IP allocation involves three major stages: `candidate pool acquisition`, `candidate pool filtering`, and `candidate pool sorting`.

- Candidate pool acquisition: Spiderpool follows a strict rule of selecting pools from **high to low priority**. It identifies all pools that match **the high priority rules** and marks them as candidates for further consideration.

- Candidate pool filtering: Spiderpool applies filtering mechanisms such as affinity to carefully select the appropriate candidate pools from the available options. This ensures that specific requirements or complex usage scenarios are satisfied.

- Candidate pool sorting: in cases where multiple candidate pools exist, Spiderpool sorts them based on the priority rules defined in the SpiderIPPool object. IP addresses are then allocated sequentially, starting from the pool with available addresses.

## Candidate pool acquisition

Spiderpool offers a variety of pool selection rules when assigning IP addresses to Pods. The selection process strictly adheres to a **high to low priority** order. The following rules are listed in **descending order of priority**, and if multiple rules apply at the same time, the preceding rule will **overwrite** the subsequent one.

- The 1st priority: SpiderSubnet annotation

    The SpiderSubnet resource represents a collection of IP addresses. When an application requires a fixed IP address, the application administrator needs to inform their platform counterparts about the available IP addresses and routing attributes. However, as they belong to different operational departments, this process becomes cumbersome, resulting in complex workflows for creating each application. To simplify this, Spiderpool's SpiderSubnet feature automatically allocates IP addresses from subnets to IPPool and assigns fixed IP addresses to applications. This greatly reduces operational costs. When creating an application, you can use the `ipam.spidernet.io/subnets` or `ipam.spidernet.io/subnet` annotation to specify the Subnet. This allows for the automatic creation of an IP pool by randomly selecting IP addresses from the subnet, which can then be allocated as fixed IPs for the application. For more details, please refer to [SpiderSubnet](../usage/spider-subnet.md).

- The 2nd priority: SpiderIPPool annotation

    Different IP addresses within a Subnet can be stored in separate instances of IPPool (Spiderpool ensures that there is no overlap between the address sets of IPPools). The size of the IP collection in SpiderIPPool can vary based on requirements. This design feature is particularly beneficial when dealing with limited IP address resources in the Underlay network. When creating an application, the SpiderIPPool annotation `ipam.spidernet.io/ippools` or `ipam.spidernet.io/ippool` can be used to bind different IPPools or share the same IPPool. This allows all applications to share the same Subnet while maintaining "micro-isolation". For more details, please refer to [SpiderIPPool annotation](../reference/annotation.md).

- The 3th priority: namespace default IP pool

    By setting the annotation `ipam.spidernet.io/default-ipv4-ippool` or `ipam.spidernet.io/default-ipv6-ippool` in the namespace, you can specify the default IP pool. When creating an application within that tenant, if there are no other higher-priority pool rules, it will attempt to allocate an IP address from the available candidate pools for that tenant. For more details, please refer to [Namespace Annotation](../reference/annotation.md).

- The fourth priority: CNI configuration file

    The global CNI default pool can be set by configuring the `default_ipv4_ippool` and `default_ipv6_ippool` fields in the CNI configuration file. Multiple IP pools can be defined as alternative pools. When an application uses this CNI configuration network and invokes Spiderpool, each application replica is sequentially assigned an IP address according to the order of elements in the "IP pool array". In scenarios where nodes belong to different regions or data centers, if the node where an application replica scheduled matches the node affinity rule of the first IP pool, the Pod obtains an IP from that pool. If it doesn't meet the criteria, Spiderpool attempts to assign an IP from the alternative pools until all options have been exhausted. For more information, please refer to [CNI Configuration](../reference/plugin-ipam.md).

- The fifth priority: cluster's default IP pool

    Within the SpiderIPPool CR object, setting the **spec.default** field to `true` designates the pool as the cluster's default IP pool (default value is `false`). For more information, please refer to [Cluster's Default IP Pool](../reference/crd-spiderippool.md).

## Candidate pool filtering

To determine the availability of candidate IP pools for IPv4 and IPv6, Spiderpool filters them using the following rules:

- IP pools in the `terminating` state are filtered out.

- The `spec.disable` field of an IP pool indicates its availability. A value of `false` means the IP pool is not usable.

- Check if the `IPPool.Spec.NodeName` and `IPPool.Spec.NodeAffinity` match the Pod's scheduling node. Mismatching values result in filtering out the IP pool.

- Check if the `IPPool.Spec.NamespaceName` and `IPPool.Spec.NamespaceAffinity` match the Pod's namespace. Mismatching values lead to filtering out the IP pool.

- Check if the `IPPool.Spec.NamespaceName` matches the Pod's `matchLabels`. Mismatching values lead to filtering out the IP pool.

- Check if the `IPPool.Spec.MultusName` matches the current NIC Multus configuration of the Pod. If there is no match, the IP pool is filtered out.

- Check if all IPs within the IP pool are included in the IPPool instance's `exclude_ips` field. If it is, the IP pool is filtered out.

- Check if all IPs in the pool are reserved in the ReservedIP instance. If it is, the IP pool is filtered out.

- An IP pool will be filtered out if its available IP resources are exhausted.

## Candidate pool sorting

After filtering the candidate pools, Spiderpool may have multiple pools remaining. To determine the order of IP address allocation, Spiderpool applies custom priority rules to sort these candidates. IP addresses are then selected from the pools with available IPs in the following manner:

- IP pool resources with the `IPPool.Spec.PodAffinity` property are given the highest priority.

- IPPool resources with either the `IPPool.Spec.NodeName` or `IPPool.Spec.NodeAffinity` property are given the secondary priority. The `NodeName` takes precedence over `NodeAffinity`.

- Following that, IP pool resources with either the `IPPool.Spec.NamespaceName` or `IPPool.Spec.NamespaceAffinity` property maintain the third-highest priority. The `NamespaceName` takes precedence over `NamespaceAffinity`.

- IP pool resources with the `IPPool.Spec.MultusName` property receive the lowest priority.

> Here are some simple instances to describe this rule.
>
> 1. *IPPoolA* with properties `IPPool.Spec.PodAffinity` and `IPPool.Spec.NodeName` has higher priority than *IPPoolB* with single affinity property `IPPool.Spec.PodAffinity`.
> 2. *IPPoolA* with single property `IPPool.Spec.PodAffinity` has higher priority than *IPPoolB* with properties `IPPool.Spec.NodeName` and `IPPool.Spec.NamespaceName`.
> 3. *IPPoolA* with properties `IPPool.Spec.PodAffinity` and `IPPool.Spec.NodeName` has higher priority than *IPPoolB* with properties `IPPool.Spec.PodAffinity`,`IPPool.Spec.NamespaceName` and `IPPool.Spec.MultusName`.
>
> If a Pod belongs to StatefulSet, IP addresses that meet the aforementioned rules will be allocated with priority. When a Pod is restarted, it will attempt to reuse the previously assigned IP address.
