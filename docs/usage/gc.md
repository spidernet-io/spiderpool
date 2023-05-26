# Reclaim IP

## Description

The spiderpool owns an IP garbage collection mechanism, it helps to clean up leaked IPs once CNI cmdDel failed.

## Enable IP GC support

Check k8s deployment `spiderpool-controller` environment property `SPIDERPOOL_GC_IP_ENABLED` whether is already set to `true` or not. (It would be enabled by default)

```shell
kubectl edit deploy spiderpool-controller -n kube-system
```

## Design

The spiderpool-controller uses `pod informer` and regular interval `scan all SpiderIPPool` to clean up leaked IPs and corresponding SpiderEndpoint object.
We used a memory cache to record the Pods object that the corresponding IPs and SpiderEndpoint objects should be cleaned.

Here are several cases that we will release IP:

* Pod was `deleted`, except StatefulSet restarts its Pod situation.

* Pod is `Terminating`, we will release its IPs after `pod.DeletionGracePeriodSeconds`, you can set environment `AdditionalGraceDelay`(default 0 seconds) to add delay duration. you can also determine whether gc `Terminating` status Pod or not with environment `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`. (It would be enabled by default).
 There are 2 situation that this env may help: 1. If one node downtime in your cluster, you must reply on the IP GC to release the IPs. 2. In some underlay mode, if you do not release one IP with terminating Pod and the new Pod cannot fetch available IP to start because of the IP resources shortage.
 But there's a special situation we should watch out for: if the node lost the connection with the master API server due to the node Interface or network issues, the Pod network may also good and still serves well. In this case, if we release its IPs and allocate them to other Pods, this will lead to IP conflict.

* Pod is `Succeeded` or `Failed` phase, we'll clean the Pod's IPs after `pod.DeletionGracePeriodSeconds`, you can set environment `AdditionalGraceDelay`(default 0 seconds) to add delay duration.

* SpiderIPPool allocated IP corresponding Pod is not exist in the Kubernetes, except StatefulSet restarts its Pod situation.

* The Pod UID is different from the SpiderIPPool IP allocation Pod UID.

## Notice

* The spiderpool controller owns multiple replicas and uses leader election, and the IP Garbage collection `pod informer` only serves for `Master`.
  But every backup will use `scan all SpiderIPPool` to release the leaked IPs that should be cleaned up immediately, it wouldn't trace the Pod into memory cache with the upper Pod status cases.

* We can change tracing Pod `AdditionalGraceDelay` with the environment `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY`. (default 5 seconds)

* If one node broke in your cluster, the IP GC will forcibly release the unreachable Pod corresponding IPs if you enabled env `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`.
And there's one rare situation that your Pod is still alive even after Pod `DeletionGracePeriod` duration, the IP GC will still forcibly release the unreachable Pod corresponding IPs.
For these two cases, we recommend the Main CNI has the ability to check IP conflict.
The [Veth](https://github.com/spidernet-io/plugins) plugin implements this feature already, you can use it to collaborate with `Macvlan` or `SR-IOV` Main CNI.
