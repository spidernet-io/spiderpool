# Resource Reclaim

***

## IP collection

***

### context

When a pod is normally deleted , the CNI plugin will be called to clean IP on Pod interface and make IP free on IPAM database.
This make sure all IP is managed correctly and no leaked IP issue.

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

The spiderpool controller takes charge of this responsibility. There is some environment to set its reclaim behaves.

// TODO (Icarus9913), describe the environment

## ippool collection

***

To prevent IP from leaking when ippool resource is deleted, the spiderpool has some rules:

* For an ippool, if there is still IP taken by pods, the spiderpool uses webhook to reject deleting request of ippool resource.

* For a deleting ippool, the IPAM plugin will stop assigning IP from it, but could release IP from it.

* The ippool is set a finalizer by the spiderpool controller. After the ippool goes to be deleting status, the spiderpool controller will remove the finalizer when all IP in the ippool is free.
