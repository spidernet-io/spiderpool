# Reclaim IP

**English** | [**简体中文**](./gc-zh_CN.md)

Spiderpool has an IP garbage collection mechanism that helps to clean up leaked IPs once CNI cmdDel fails.

## Enable IP GC Support

Check the `SPIDERPOOL_GC_IP_ENABLED` environment property of the `spiderpool-controller` Kubernetes deployment to see if it is already set to `true`. (It is enabled by default.)

```shell
kubectl edit deploy spiderpool-controller -n kube-system
```

## Design

The spiderpool-controller uses `pod informer` and regular interval `scan all SpiderIPPool` to clean up leaked IPs and corresponding SpiderEndpoint objects.
We used a memory cache to record the Pods object that the corresponding IPs and SpiderEndpoint objects should be cleaned up.

Here are several cases in which we release IP:

* Pod was `deleted`, except StatefulSet restarts its Pod situation.

* Pod is `Terminating`, we will release its IPs after `pod.DeletionGracePeriodSeconds`, you can set environment `AdditionalGraceDelay`(default 0 seconds) to add delay duration. you can also determine whether gc `Terminating` status Pod or not with environment `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`. (It would be enabled by default). There are two cases that this env may help:
    
    1. If one node encounters downtime in your cluster, you must rely on the IP GC to release the IPs.
    2. In some underlay mode, if you do not release one IP with terminating Pod and the new Pod cannot fetch available IP to start because of the IP resources shortage.

    But there's a special case we should watch out for: if the node lost the connection with the master API server due to the node Interface or network issues, the Pod network may also be good and still serves well. In this case, if we release its IPs and allocate them to other Pods, this will lead to IP conflict.

* Pod is in the `Succeeded` or `Failed` phase, we'll clean the Pod's IPs after `pod.DeletionGracePeriodSeconds`, you can set the `AdditionalGraceDelay`(default 0 seconds) environment variable to add delay duration.

* SpiderIPPool allocated IP corresponding Pod does not exist in Kubernetes, except StatefulSet restarts its Pod situation.

* The Pod UID is different from the SpiderIPPool IP allocation Pod UID.

## Notice

* The `spiderpool-controller` has multiple replicas and uses leader election. The IP Garbage Collection `pod informer` only serves the `Master`.
  However, every backup will use `scan all SpiderIPPool` to release the leaked IPs that should be cleaned up immediately. It will not trace the Pod into memory cache with the upper Pod status cases.

* We can change tracing Pod `AdditionalGraceDelay` with the environment `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY`(default 5 seconds).

* If one node breaks in your cluster, the IP GC will forcibly release the unreachable Pod corresponding IPs if you enable the `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED` environment variable.
  There is also a rare situation where your Pod is still alive even after the `DeletionGracePeriod` duration. The IP GC will still forcibly release the unreachable Pod corresponding IPs.
  For these two cases, we recommend that the Main CNI has the ability to check for IP conflicts.
  The [Veth](https://github.com/spidernet-io/plugins) plugin already implements this feature, and you can use it in collaboration with `Macvlan` or `SR-IOV` CNI.
