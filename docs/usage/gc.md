# Reclaim IP

**English** | [**简体中文**](./gc-zh_CN.md)

## Introduce

In Kubernetes, garbage collection (Garbage Collection, GC for short) is very important for the recycling of IP addresses. The availability of IP addresses is critical to whether a Pod can start successfully. The GC mechanism can automatically reclaim these unused IP addresses, avoiding waste of resources and exhaustion of IP addresses. This article will introduce Spiderpool's excellent GC capabilities.

## Project Functions

The IP addresses assigned to Pods are recorded in IPAM, but these Pods no longer exist in the Kubernetes cluster. These IPs can be called `zombie IPs`. Spiderpool can recycle `zombie IPs`. Its implementation principle is as follows :

When `deleting Pod` in the cluster, but due to problems such as `network exception` or `cni binary crash`, the call to `cni delete` fails, resulting in the IP address not being reclaimed by cni.

- In failure scenarios such as `cni delete failure`, if a Pod that has been assigned an IP is destroyed, but the IP address is still recorded in the IPAM, a phenomenon of zombie IP is formed. For this kind of problem, Spiderpool will automatically recycle these zombie IP addresses based on the cycle and event scanning mechanism.

After a node goes down unexpectedly, the Pod in the cluster is permanently in the `deleting` state, and the IP address occupied by the Pod cannot be released.

- For a Pod in `Terminating` state, Spiderpool will automatically release its IP address after the Pod's `spec.terminationGracePeriodSecond`. This feature can be controlled by the environment variable `SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED`. This capability can be used to solve the failure scenario of `unexpected node downtime`.
