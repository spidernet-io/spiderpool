# Resource Reclaim

## IP garbage collection

### Context

When a pod is normally deleted, the CNI plugin will be called to clean IP on a pod interface and make IP free on IPAM database.
This can make sure all IPs are managed correctly and no IP leakage issue occurs.

But on cases, it may go wrong and IP of IPAM database is still marked as used by a nonexistent pod.

when some errors happened, the CNI plugin is not called correctly when pod deletion. This could happen like cases:

* When a CNI plugin is called, its network communication goes wrong and fails to release IP.

* The container runtime goes wrong and fails to call CNI plugin.

* A node breaks down and then always can not recover, the api-server makes pods of the breakdown node to be `deleting` status, but the CNI plugin fails to be called.

BTW, this fault could be simply simulated by removing the CNI binary on a host when pod deletion.

This issue will make a bad result:

* the new pod may fail to run because the expected IP is still occupied.

* the IP resource is exhausted gradually although the actual number of pods does not grow.

Some CNI or IPAM plugins could not handle this issue. For some CNIs, the administrator self needs to find the IP with this issue and use a CLI tool to reclaim them.
For some CNIs, it runs an interval job to find the IP with this issue and not reclaim them in time. For some CNIs, there is not any mechanism at all to fix the IP issue.

### Solution

For some CNIs, its IP CIDR is big enough, so the leaked IP issue is not urgent.
For Spiderpool, all IP resources are managed by administrator, and an application will be bound to a fixed IP, so the IP reclaim can be finished in time.

The spiderpool controller takes charge of this responsibility. For more details, please refer to [IP GC](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/gc.md).

## SpiderIPPool garbage collection

To prevent IP from leaking when the ippool resource is deleted, Spiderpool has some rules:

* For an ippool, if IP still taken by pods, Spiderpool uses webhook to reject deleting request of the ippool resource.

* For a deleting ippool, the IPAM plugin will stop assigning IP from it, but could release IP from it.

* The ippool sets a finalizer by the spiderpool controller once it is created. After the ippool goes to be `deleting` status,
the spiderpool controller will remove the finalizer when all IPs in the ippool are free, then the ippool object will be deleted.

## SpiderEndpoint garbage collection

Once a pod is created and gets IPs from `SpiderIPPool`, Spiderpool will create a corresponding `SpiderEndpoint` object at the same time.
It will take a finalizer (except the StatefulSet pod) and will be set to `OwnerReference` with the pod.

When a pod is deleted, Spiderpool will release its IPs with the recorded data by a corresponding `SpiderEndpoint` object,
then spiderpool controller will remove the `Current` data of SpiderEndpoint object and remove its finalizer.
(For the StatefulSet `SpiderEndpoint`, Spiderpool will delete it directly if its `Current` data was cleaned up)
