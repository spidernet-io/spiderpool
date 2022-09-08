# Resource Reclaim

## IP garbage collection

### context

When a pod is normally deleted, the CNI plugin will be called to clean IP on a pod interface and make IP free on IPAM database.
This can make sure all IPs are managed correctly and no IP leakage issue occurs.

But on cases, it may go wrong and IP of IPAM database is still recorded to be used by a non-existed pod.

when some error happened, the CNI plugin is not called correctly when pod deletion. This could happen like cases:

* When CNI plugin is called, its network communication goes wrong and fails to release IP.

* The container runtime goes wrong and fail to call CNI plugin.

* A node is breakdown and then always not recovery, the api-server makes pods of the breakdown node to be deleting status, but the CNI plugin fails to be called.

BTW, this fault could be simply simulated by removing CNI binary on host when pod deletion.

This issue will make bad result:

* the new pod maybe fail to run because the expected IP is still occupied.

* the IP resource is exhausted gradually although the actual pod number does not grow.

Some CNI or IPAM plugins, they could not handle this issue. For some CNI, the administrator self needs to find issue IP and use CLI tool to reclaim them.
For some CNI, it runs an interval job to find the issue IP and not reclaim them in time. For some CNI, there is not any mechanism at all, to fix these issue IP.

### solution

For some CNI, its IP CIDR is big enough, so the leaked IP issue is not urgent.
For spiderpool, all IP resource is managed by Administrator, and application will be bound to fixed IP, so the IP reclaim must be finished in time.

The spiderpool controller takes charge of this responsibility. For more details, please refer to [IP GC](https://github.com/spidernet-io/spiderpool/blob/main/pkg/gcmanager/README.md)

## SpiderIPPool garbage collection

To prevent IP from leaking when ippool resource is deleted, the spiderpool has some rules:

* For an ippool, if there is still IP taken by pods, the spiderpool uses webhook to reject deleting request of ippool resource.

* For a deleting ippool, the IPAM plugin will stop assigning IP from it, but could release IP from it.

* The ippool is set a finalizer by the spiderpool controller once it was created. After the ippool goes to be deleting status,
the spiderpool controller will remove the finalizer when all IP in the ippool is free, then the ippool object will be deleted.

## SpiderEndpoint garbage collection

Once a pod was created and got IPs from SpiderIPPool, the Spiderpool will create a corresponding SpiderEndpoint object at the same time,
and it will take a finalizer (except StatefulSet pod) and will be set `OwnerReference` with the pod.

When a pod was deleted, spiderpool will release its IPs with the corresponding SpiderEndpoint object recorded data,
then spiderpool controller will remove the SpiderEndpoint object `Current` data and remove its finalizer.
(For StatefulSet SpiderEndpoint, the spiderpool will delete it directly if its `Current` data was cleaned up)
