# Spiderpool IP garbage collection

## Description

The spiderpool owns a IP garbage collection mechanism, it helps to clean up leaked IPs once CNI cmdDel failed.

### Enable IP GC support

Check k8s deployment `spiderpool-controller` environment property `SPIDERPOOL_GC_IP_ENABLED` whether is already set to `true` or not. (It would be enabled by default)

We can also control whether to trace `Terminating` status phase Pod or not with environment `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`. (It would be enabled by default)

```shell
kubectl edit deploy spiderpool-controller -n kube-system
```

### Design

The spiderpool uses `pod informer` and regular interval `scan all SpiderIPPool` to clean up leaked IPs and corresponding SpiderEndpoint object.
We used a memory cache to record Pods which their corresponding IPs and SpiderEndpoint objects should be cleaned.

Here are several cases that the pod should be recorded:

* pod was `deleted` (spiderpool will surely clean up its IPs and SpiderEndpoint object immediately, except StatefulSet just restarts its pod case)

* pod is `Terminating`, spiderpool will begin to trace it, and after `pod DeletionGracePeriodSeconds` + `AdditionalGraceDelay`(default 5 seconds) to clean them.

* pod is `Succeeded` or `Failed`, CNI cmdDel will be called after a pod turns to `Succeeded` or `Failed` status.
And spiderpool controller will record it with pod `containerStatuses.state.terminated.finishedAt` time and  after `pod DeletionGracePeriodSeconds` + `AdditionalGraceDelay`(default 5 seconds) to clean them.

The spiderpool `pod informer` uses kubernetes informer mechanism to build a cache data with the upper cases.

For spiderpool `scan all SpiderIPPool`, it will traverse the SpiderIPPoolList to check each IP whether is used by a real pod or not and decide to clean it up immediately.
Once the IP corresponding pod is alive but the container ID is different, the IP and SpiderEndpoint would be cleaned up immediately either.
For those container ID is same and pod is alive situation, it will build a cache data depending on the pod status whether belongs to the upper cases.

## Notice

* The spiderpool controller owns multiple replicas and uses leader election, and the IP Garbage collection `pod informer` only serves for `Master`.
But every backup will use `scan all SpiderIPPool` to release the leaked IPs that should be cleaned up immediately, it wouldn't trace pod into memory cache with the upper pod status cases.

* We can change the `scan all SpiderIPPool` regular interval duration with environment `SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION`. (default 10 minutes)

* We can change tracing pod `AdditionalGraceDelay` with environment `SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY`. (default 5 seconds)
