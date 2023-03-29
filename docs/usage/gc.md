# Reclaim IP

## Description

The spiderpool owns an IP garbage collection mechanism, it helps to clean up leaked IPs once CNI cmdDel failed.

## Enable IP GC support

Check k8s deployment `spiderpool-controller` environment property `SPIDERPOOL_GC_IP_ENABLED` whether is already set to `true` or not. (It would be enabled by default)

We can also control whether to trace `Terminating` status phase Pod or not with environment `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`. (It would be enabled by default)

```shell
kubectl edit deploy spiderpool-controller -n kube-system
```

## Design


The spiderpool-controller uses `pod informer` and regular interval `scan all SpiderIPPool` to clean up leaked IPs and corresponding SpiderEndpoint object.
We used a memory cache to record the Pods object that the corresponding IPs and SpiderEndpoint objects should be cleaned.

Here are several cases that we will release IP:

* pod was `deleted`, except StatefulSet restarts its pod situation.

* pod is `Terminating`, `Succeeded` or `Failed` phase, we'll clean the pod's IPs after `pod DeletionGracePeriodSeconds` + `AdditionalGraceDelay`(default 0 seconds).

* SpiderIPPool allocated IP corresponding pod is not exist in the Kubernetes, except StatefulSet restarts its pod situation.

* The pod UID is different from the SpiderIPPool IP allocation pod UID.

## Notice

* The spiderpool controller owns multiple replicas and uses leader election, and the IP Garbage collection `pod informer` only serves for `Master`.
  But every backup will use `scan all SpiderIPPool` to release the leaked IPs that should be cleaned up immediately, it wouldn't trace the pod into memory cache with the upper pod status cases.

* We can change tracing pod `AdditionalGraceDelay` with the environment `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY`. (default 5 seconds)

* If one node broke in your cluster, the IP GC will forcibly release the unreachable pod corresponding IPs if you enabled env `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`. 
And there's one rare situation that your pod is still alive even after pod `DeletionGracePeriod` duration, the IP GC will still forcibly release the unreachable pod corresponding IPs.
For these two cases, we recommend the Main CNI has the ability to check IP conflict. 
The [Veth](https://github.com/spidernet-io/plugins) plugin implements this feature already, you can use it to collaborate with `Macvlan` or `SR-IOV` Main CNI.
